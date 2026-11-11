package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const maxConcurrentChecks = 20

// Checker probes every service once per its interval.
//
// Each second it lists services and tries to take a Redis lock per service with
// TTL = interval. Whoever gets the lock runs the check; everyone else skips it.
// New services have no lock yet, so they are checked on the next tick.
type Checker struct {
	store *Store
	cache *Cache
	http  *http.Client
	sem   chan struct{}
	wg    sync.WaitGroup
}

func NewChecker(store *Store, cache *Cache) *Checker {
	return &Checker{
		store: store,
		cache: cache,
		http:  &http.Client{Timeout: 10 * time.Second},
		sem:   make(chan struct{}, maxConcurrentChecks),
	}
}

// Run blocks until ctx is cancelled and in-flight checks have finished.
func (c *Checker) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			c.wg.Wait()
			return
		case <-t.C:
			c.tick(ctx)
		}
	}
}

func (c *Checker) tick(ctx context.Context) {
	services, err := c.store.ListServices(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("list services", "err", err)
		}
		return
	}
	for _, svc := range services {
		won, err := c.cache.TryLock(ctx, svc.ID, time.Duration(svc.IntervalSeconds)*time.Second)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("acquire check lock", "service", svc.ID, "err", err)
			}
			continue
		}
		if !won {
			continue
		}
		select {
		case c.sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		c.wg.Add(1)
		go func(s Service) {
			defer c.wg.Done()
			defer func() { <-c.sem }()
			c.check(ctx, s)
		}(svc)
	}
}

func (c *Checker) check(ctx context.Context, svc Service) {
	res := c.probe(ctx, svc)
	if ctx.Err() != nil {
		return // shutting down; don't record a bogus "context canceled" failure
	}
	res.CheckedAt = time.Now().UTC()

	// Persist first. If the service was deleted mid-check, the foreign key rejects
	// the insert and we skip the cache write.
	if err := c.store.InsertCheck(ctx, &res); err != nil {
		slog.Warn("store check", "service", svc.ID, "err", err)
		return
	}
	if err := c.cache.SetStatus(ctx, svc.ID, res.Status()); err != nil {
		slog.Warn("cache status", "service", svc.ID, "err", err)
	}
}

func (c *Checker) probe(ctx context.Context, svc Service) Check {
	res := Check{ServiceID: svc.ID}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.URL, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("User-Agent", "status-tracker/1.0")

	start := time.Now()
	resp, err := c.http.Do(req)
	res.LatencyMS = int(time.Since(start).Milliseconds())
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	res.StatusCode = resp.StatusCode
	res.OK = resp.StatusCode == svc.ExpectedStatus
	if !res.OK {
		res.Error = fmt.Sprintf("got status %d, expected %d", resp.StatusCode, svc.ExpectedStatus)
	}
	return res
}

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

//go:embed web
var webFS embed.FS

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, getenv("DATABASE_URL", "postgres://status:status@localhost:5432/status?sslmode=disable"))
	if err != nil {
		return fmt.Errorf("postgres config: %w", err)
	}
	defer pool.Close()

	ropts, err := redis.ParseURL(getenv("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		return fmt.Errorf("redis config: %w", err)
	}
	rdb := redis.NewClient(ropts)
	defer rdb.Close()

	store, cache := NewStore(pool), NewCache(rdb)

	if err := waitFor(ctx, "postgres", store.Ping); err != nil {
		return err
	}
	if err := waitFor(ctx, "redis", cache.Ping); err != nil {
		return err
	}
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}

	retentionDays, err := strconv.Atoi(getenv("RETENTION_DAYS", "7"))
	if err != nil || retentionDays < 1 {
		retentionDays = 7
	}

	// Background workers.
	var bg sync.WaitGroup
	checker := NewChecker(store, cache)
	bg.Add(2)
	go func() { defer bg.Done(); checker.Run(ctx) }()
	go func() { defer bg.Done(); pruneLoop(ctx, store, time.Duration(retentionDays)*24*time.Hour) }()

	// HTTP server.
	srv := &http.Server{
		Addr:              getenv("ADDR", ":8080"),
		Handler:           NewAPI(store, cache).Routes(webRoot),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	slog.Info("listening", "addr", srv.Addr)
	var serveErr error
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		serveErr = err
		stop() // triggers the shutdown goroutine and stops the workers
	}
	<-shutdownDone
	bg.Wait()
	slog.Info("stopped")
	return serveErr
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// waitFor retries a dependency ping so the app can start alongside `docker compose up`.
func waitFor(ctx context.Context, name string, ping func(context.Context) error) error {
	var err error
	for i := 0; i < 30; i++ {
		if err = ping(ctx); err == nil {
			return nil
		}
		slog.Info("waiting for dependency", "name", name, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("%s not reachable: %w", name, err)
}

// pruneLoop deletes old check rows hourly so the checks table doesn't grow forever.
func pruneLoop(ctx context.Context, store *Store, retention time.Duration) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := store.PruneChecks(ctx, retention)
			if err != nil {
				slog.Warn("prune checks", "err", err)
			} else if n > 0 {
				slog.Info("pruned old checks", "rows", n)
			}
		}
	}
}

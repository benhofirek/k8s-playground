package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis does two jobs here:
//  1. caches each service's latest Status so list/status reads skip Postgres
//  2. holds a per-service lock whose TTL equals the check interval, so only one
//     instance probes a service per interval even when several are running
const statusTTL = 24 * time.Hour

type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func (c *Cache) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

func statusKey(id int64) string { return "status:" + strconv.FormatInt(id, 10) }
func lockKey(id int64) string   { return "lock:check:" + strconv.FormatInt(id, 10) }

func (c *Cache) SetStatus(ctx context.Context, id int64, st Status) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, statusKey(id), b, statusTTL).Err()
}

// GetStatus returns nil (and no error) on a cache miss.
func (c *Cache) GetStatus(ctx context.Context, id int64) (*Status, error) {
	b, err := c.rdb.Get(ctx, statusKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st Status
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// GetStatuses fetches many statuses in one round trip. Misses are absent from the map.
func (c *Cache) GetStatuses(ctx context.Context, ids []int64) (map[int64]Status, error) {
	out := make(map[int64]Status, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = statusKey(id)
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return out, err
	}
	for i, v := range vals {
		s, ok := v.(string)
		if !ok {
			continue
		}
		var st Status
		if json.Unmarshal([]byte(s), &st) == nil {
			out[ids[i]] = st
		}
	}
	return out, nil
}

func (c *Cache) Forget(ctx context.Context, id int64) error {
	return c.rdb.Del(ctx, statusKey(id), lockKey(id)).Err()
}

// TryLock reports whether this caller won the right to check the service now.
func (c *Cache) TryLock(ctx context.Context, id int64, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, lockKey(id), 1, ttl).Result()
}

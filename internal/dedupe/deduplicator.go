package dedupe

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	rkeys "github.com/O-Schema/oschema/internal/redis"
)

// RedisDeduplicator uses Redis SET NX for atomic deduplication.
type RedisDeduplicator struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisDeduplicator(rdb *redis.Client, ttl time.Duration) *RedisDeduplicator {
	return &RedisDeduplicator{rdb: rdb, ttl: ttl}
}

func (d *RedisDeduplicator) IsDuplicate(ctx context.Context, source, externalID string) (bool, error) {
	key := rkeys.DedupeKey(source, externalID)
	set, err := d.rdb.SetNX(ctx, key, "1", d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("dedupe check: %w", err)
	}
	return !set, nil
}

// Clear removes a dedupe key. Used to undo dedup when enqueue fails.
func (d *RedisDeduplicator) Clear(ctx context.Context, source, externalID string) {
	key := rkeys.DedupeKey(source, externalID)
	d.rdb.Del(ctx, key)
}

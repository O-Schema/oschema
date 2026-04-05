package dedupe

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Deduplicator checks whether an event has already been processed.
type Deduplicator interface {
	IsDuplicate(ctx context.Context, source, externalID string) (bool, error)
}

// RedisDeduplicator uses Redis SET NX for atomic deduplication.
type RedisDeduplicator struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewRedisDeduplicator creates a deduplicator backed by Redis SET NX with a TTL.
func NewRedisDeduplicator(rdb *redis.Client, ttl time.Duration) *RedisDeduplicator {
	return &RedisDeduplicator{rdb: rdb, ttl: ttl}
}

func (d *RedisDeduplicator) IsDuplicate(ctx context.Context, source, externalID string) (bool, error) {
	key := fmt.Sprintf("dedupe:%s:%s", source, externalID)
	set, err := d.rdb.SetNX(ctx, key, "1", d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("dedupe check: %w", err)
	}
	// SetNX returns true if the key was set (first time), false if it already existed
	return !set, nil
}

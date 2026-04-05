package dedupe

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestDeduplicatorFirstSeen(t *testing.T) {
	rdb := setupMiniRedis(t)
	d := NewRedisDeduplicator(rdb, 24*time.Hour)

	dup, err := d.IsDuplicate(context.Background(), "shopify", "12345")
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if dup {
		t.Error("first event should not be duplicate")
	}
}

func TestDeduplicatorSecondSeen(t *testing.T) {
	rdb := setupMiniRedis(t)
	d := NewRedisDeduplicator(rdb, 24*time.Hour)
	ctx := context.Background()

	_, _ = d.IsDuplicate(ctx, "shopify", "12345")
	dup, err := d.IsDuplicate(ctx, "shopify", "12345")
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if !dup {
		t.Error("second event should be duplicate")
	}
}

func TestDeduplicatorDifferentSources(t *testing.T) {
	rdb := setupMiniRedis(t)
	d := NewRedisDeduplicator(rdb, 24*time.Hour)
	ctx := context.Background()

	_, _ = d.IsDuplicate(ctx, "shopify", "12345")
	dup, err := d.IsDuplicate(ctx, "stripe", "12345")
	if err != nil {
		t.Fatalf("IsDuplicate: %v", err)
	}
	if dup {
		t.Error("same ID from different source should not be duplicate")
	}
}

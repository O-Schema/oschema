package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/pkg/event"
)

func newTestEvent(source, id string) *event.Event {
	return &event.Event{
		ID:         id,
		Source:     source,
		Version:    "1.0",
		Type:       "test.event",
		ExternalID: "ext-" + id,
		Timestamp:  time.Now().UTC(),
		Data:       map[string]any{"key": "value"},
		Raw:        map[string]any{"raw_key": "raw_value"},
	}
}

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	evt := newTestEvent("shopify", "evt-1")

	if err := s.Save(ctx, evt); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get(ctx, evt.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != evt.ID {
		t.Errorf("ID = %q, want %q", got.ID, evt.ID)
	}

	list, err := s.List(ctx, "shopify", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List length = %d, want 1", len(list))
	}
}

func TestMemoryStoreGetNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent event")
	}
}

func TestRedisStore(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := NewRedisStore(rdb)
	ctx := context.Background()
	evt := newTestEvent("shopify", "evt-redis-1")

	if err := s.Save(ctx, evt); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get(ctx, evt.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != evt.ID {
		t.Errorf("ID = %q, want %q", got.ID, evt.ID)
	}

	list, err := s.List(ctx, "shopify", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List length = %d, want 1", len(list))
	}
}

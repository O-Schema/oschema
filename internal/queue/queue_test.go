package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/pkg/event"
)

func setupQueue(t *testing.T) (*RedisQueue, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := NewRedisQueue(rdb, Config{
		Stream:     "oschema:queue",
		Group:      "test-workers",
		Consumer:   "test-1",
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	})
	return q, mr
}

func TestEnqueueAndDequeue(t *testing.T) {
	q, _ := setupQueue(t)
	ctx := context.Background()

	evt := &event.Event{
		ID:     "evt-1",
		Source: "shopify",
		Type:   "order.created",
		Data:   map[string]any{"key": "value"},
		Raw:    map[string]any{"raw": "data"},
	}

	if err := q.Enqueue(ctx, evt); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	msgs, err := q.Dequeue(ctx, 1, time.Second)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Dequeue count = %d, want 1", len(msgs))
	}

	var got event.Event
	if err := json.Unmarshal([]byte(msgs[0].Payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "evt-1" {
		t.Errorf("ID = %q, want %q", got.ID, "evt-1")
	}
}

func TestAck(t *testing.T) {
	q, _ := setupQueue(t)
	ctx := context.Background()

	evt := &event.Event{ID: "evt-2", Source: "test", Data: map[string]any{}, Raw: map[string]any{}}
	_ = q.Enqueue(ctx, evt)

	msgs, _ := q.Dequeue(ctx, 1, time.Second)
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}

	if err := q.Ack(ctx, msgs[0].StreamID); err != nil {
		t.Fatalf("Ack: %v", err)
	}
}

func TestDeadLetter(t *testing.T) {
	q, _ := setupQueue(t)
	ctx := context.Background()

	evt := &event.Event{ID: "evt-dl", Source: "test", Data: map[string]any{}, Raw: map[string]any{}}
	if err := q.DeadLetter(ctx, evt, "max retries exceeded"); err != nil {
		t.Fatalf("DeadLetter: %v", err)
	}
}

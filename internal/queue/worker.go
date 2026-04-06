package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/O-Schema/oschema/internal/store"
	"github.com/O-Schema/oschema/pkg/event"
)

// Worker processes events from the queue and stores them.
type Worker struct {
	queue *RedisQueue
	store store.EventStore
}

func NewWorker(q *RedisQueue, s store.EventStore) *Worker {
	return &Worker{queue: q, store: s}
}

// Run starts the worker loop. It blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("worker started (consumer=%s)", w.queue.config.Consumer)
	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return
		default:
		}

		msgs, err := w.queue.Dequeue(ctx, 10, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("dequeue error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			w.handleMessage(ctx, msg)
		}
	}
}

func (w *Worker) handleMessage(ctx context.Context, msg Message) {
	err := w.processMessage(ctx, msg.Payload)
	if err == nil {
		// Success — ack immediately
		if ackErr := w.queue.Ack(ctx, msg.StreamID); ackErr != nil {
			log.Printf("ack error: %v", ackErr)
		}
		return
	}

	log.Printf("process error (stream_id=%s): %v", msg.StreamID, err)

	// Check delivery count from Redis XPENDING to decide retry vs dead-letter.
	// Since XPENDING per-message is expensive, we use a simpler approach:
	// track attempts via a Redis counter keyed by stream ID.
	attempt := w.incrementAttempt(ctx, msg.StreamID)
	maxRetries := w.queue.MaxRetries()
	if maxRetries <= 0 {
		maxRetries = 5
	}

	if attempt < maxRetries {
		// Retry: do NOT ack — leave in pending. Apply backoff delay.
		delay := w.queue.RetryDelay(attempt)
		log.Printf("retry %d/%d for stream_id=%s (backoff=%s)", attempt, maxRetries, msg.StreamID, delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
		// Message stays unacked — will be redelivered via XREADGROUP on pending
		return
	}

	// Exhausted retries — dead letter
	log.Printf("dead-lettering stream_id=%s after %d attempts", msg.StreamID, attempt)
	var evt event.Event
	if jsonErr := json.Unmarshal([]byte(msg.Payload), &evt); jsonErr == nil {
		if dlErr := w.queue.DeadLetter(ctx, &evt, err.Error()); dlErr != nil {
			log.Printf("dead-letter error: %v", dlErr)
			return // Don't ack if dead-letter fails — will be retried
		}
	}
	// Clean up attempt counter
	w.clearAttempt(ctx, msg.StreamID)
	// Ack only after successful dead-lettering
	if ackErr := w.queue.Ack(ctx, msg.StreamID); ackErr != nil {
		log.Printf("ack error after dead-letter: %v", ackErr)
	}
}

func (w *Worker) incrementAttempt(ctx context.Context, streamID string) int {
	key := "oschema:attempts:" + streamID
	val, err := w.queue.rdb.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("attempt counter error: %v", err)
		return 1
	}
	// Set TTL on first attempt so counters don't leak
	if val == 1 {
		w.queue.rdb.Expire(ctx, key, 24*time.Hour)
	}
	return int(val)
}

func (w *Worker) clearAttempt(ctx context.Context, streamID string) {
	key := "oschema:attempts:" + streamID
	w.queue.rdb.Del(ctx, key)
}

func (w *Worker) processMessage(ctx context.Context, payload string) error {
	var evt event.Event
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}
	if err := w.store.Save(ctx, &evt); err != nil {
		return fmt.Errorf("store event: %w", err)
	}
	log.Printf("stored event id=%s source=%s type=%s", evt.ID, evt.Source, evt.Type)
	return nil
}

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/O-Schema/oschema/internal/metrics"
	rkeys "github.com/O-Schema/oschema/internal/redis"
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
	slog.Info("worker started", "consumer", w.queue.config.Consumer)
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down")
			return
		default:
		}

		msgs, err := w.queue.Dequeue(ctx, 10, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("dequeue failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		var ackIDs []string
		for _, msg := range msgs {
			if id := w.handleMessage(ctx, msg); id != "" {
				ackIDs = append(ackIDs, id)
			}
		}
		if len(ackIDs) > 0 {
			if err := w.queue.AckBatch(ctx, ackIDs...); err != nil {
				slog.Error("batch ack failed", "error", err)
			}
		}
	}
}

func (w *Worker) handleMessage(ctx context.Context, msg Message) string {
	err := w.processMessage(ctx, msg.Payload)
	if err == nil {
		return msg.StreamID // ack this one
	}

	slog.Error("process failed", "stream_id", msg.StreamID, "error", err)

	attempt := w.incrementAttempt(ctx, msg.StreamID)
	maxRetries := w.queue.MaxRetries()
	if maxRetries <= 0 {
		maxRetries = 5
	}

	if attempt < maxRetries {
		delay := w.queue.RetryDelay(attempt)
		slog.Info("retrying message", "attempt", attempt, "max_retries", maxRetries, "stream_id", msg.StreamID, "backoff", delay)
		var retryEvt event.Event
		if jsonErr := json.Unmarshal([]byte(msg.Payload), &retryEvt); jsonErr == nil {
			metrics.EventsRetried.WithLabelValues(retryEvt.Source).Inc()
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
		return "" // don't ack — will be redelivered
	}

	// Exhausted retries — dead letter
	slog.Warn("dead-lettering message", "stream_id", msg.StreamID, "attempts", attempt)
	var evt event.Event
	if jsonErr := json.Unmarshal([]byte(msg.Payload), &evt); jsonErr == nil {
		if dlErr := w.queue.DeadLetter(ctx, &evt, err.Error()); dlErr != nil {
			slog.Error("dead-letter failed", "error", dlErr)
			return "" // don't ack if dead-letter fails
		}
		metrics.EventsDeadLettered.WithLabelValues(evt.Source).Inc()
	}
	w.clearAttempt(ctx, msg.StreamID)
	return msg.StreamID // ack after successful dead-letter
}

func (w *Worker) incrementAttempt(ctx context.Context, streamID string) int {
	key := rkeys.AttemptPrefix + streamID
	val, err := w.queue.rdb.Incr(ctx, key).Result()
	if err != nil {
		slog.Error("attempt counter failed", "error", err)
		return 1
	}
	// Set TTL on first attempt so counters don't leak
	if val == 1 {
		w.queue.rdb.Expire(ctx, key, 24*time.Hour)
	}
	return int(val)
}

func (w *Worker) clearAttempt(ctx context.Context, streamID string) {
	key := rkeys.AttemptPrefix + streamID
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
	metrics.EventsStored.WithLabelValues(evt.Source).Inc()
	slog.Info("event stored", "id", evt.ID, "source", evt.Source, "type", evt.Type)
	return nil
}

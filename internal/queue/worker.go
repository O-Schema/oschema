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

		msgs, err := w.queue.Dequeue(ctx, 1, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("dequeue error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range msgs {
			if err := w.processMessage(ctx, msg.Payload); err != nil {
				log.Printf("process error (stream_id=%s): %v", msg.StreamID, err)
				var evt event.Event
				if jsonErr := json.Unmarshal([]byte(msg.Payload), &evt); jsonErr == nil {
					_ = w.queue.DeadLetter(ctx, &evt, err.Error())
				}
			}
			if ackErr := w.queue.Ack(ctx, msg.StreamID); ackErr != nil {
				log.Printf("ack error: %v", ackErr)
			}
		}
	}
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

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/pkg/event"
)

// Config holds queue configuration.
type Config struct {
	Stream     string
	Group      string
	Consumer   string
	MaxRetries int
	BaseDelay  time.Duration
}

// Message represents a dequeued message.
type Message struct {
	StreamID string
	Payload  string
}

// RedisQueue uses Redis Streams for durable event queuing.
type RedisQueue struct {
	rdb      *redis.Client
	config   Config
	initOnce sync.Once
	initErr  error
}

func NewRedisQueue(rdb *redis.Client, cfg Config) *RedisQueue {
	return &RedisQueue{rdb: rdb, config: cfg}
}

// EnsureGroup creates the consumer group if it doesn't exist. Safe to call multiple times.
func (q *RedisQueue) EnsureGroup(ctx context.Context) error {
	q.initOnce.Do(func() {
		err := q.rdb.XGroupCreateMkStream(ctx, q.config.Stream, q.config.Group, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			q.initErr = fmt.Errorf("create consumer group: %w", err)
		}
	})
	return q.initErr
}

// Enqueue adds an event to the queue stream.
func (q *RedisQueue) Enqueue(ctx context.Context, evt *event.Event) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.config.Stream,
		Values: map[string]any{"event": string(data)},
	}).Err()
}

// Dequeue reads messages from the stream via the consumer group.
func (q *RedisQueue) Dequeue(ctx context.Context, count int, block time.Duration) ([]Message, error) {
	if err := q.EnsureGroup(ctx); err != nil {
		return nil, err
	}

	streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.config.Group,
		Consumer: q.config.Consumer,
		Streams:  []string{q.config.Stream, ">"},
		Count:    int64(count),
		Block:    block,
	}).Result()

	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue: %w", err)
	}

	var msgs []Message
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			payload, ok := msg.Values["event"].(string)
			if !ok {
				continue
			}
			msgs = append(msgs, Message{
				StreamID: msg.ID,
				Payload:  payload,
			})
		}
	}
	return msgs, nil
}

// Ack acknowledges a processed message.
func (q *RedisQueue) Ack(ctx context.Context, streamID string) error {
	return q.rdb.XAck(ctx, q.config.Stream, q.config.Group, streamID).Err()
}

// DeadLetter moves a failed event to the dead letter stream.
func (q *RedisQueue) DeadLetter(ctx context.Context, evt *event.Event, reason string) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "oschema:deadletter",
		Values: map[string]any{
			"event":  string(data),
			"reason": reason,
		},
	}).Err()
}

// RetryDelay calculates exponential backoff delay for a given attempt.
func (q *RedisQueue) RetryDelay(attempt int) time.Duration {
	delay := q.config.BaseDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
	}
	return delay
}

// MaxRetries returns the configured max retries.
func (q *RedisQueue) MaxRetries() int {
	return q.config.MaxRetries
}

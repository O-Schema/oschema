package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/pkg/event"

	rkeys "github.com/O-Schema/oschema/internal/redis"
)

// RedisStore persists events using Redis Streams and a hash for lookups.
type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

func (s *RedisStore) Save(ctx context.Context, evt *event.Event) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	streamKey := rkeys.EventStreamKey(evt.Source)
	pipe := s.rdb.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 100000,
		Approx: true,
		Values: map[string]any{"event": string(data)},
	})
	pipe.HSet(ctx, rkeys.EventIndex, evt.ID, string(data))
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("store event: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, id string) (*event.Event, error) {
	data, err := s.rdb.HGet(ctx, rkeys.EventIndex, id).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("event %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	var evt event.Event
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &evt, nil
}

func (s *RedisStore) List(ctx context.Context, source string, limit int) ([]*event.Event, error) {
	streamKey := rkeys.EventStreamKey(source)
	msgs, err := s.rdb.XRevRangeN(ctx, streamKey, "+", "-", int64(limit)).Result()
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	events := make([]*event.Event, 0, len(msgs))
	for _, msg := range msgs {
		data, ok := msg.Values["event"].(string)
		if !ok {
			slog.Warn("corrupt stream entry", "stream_id", msg.ID, "reason", "missing event field")
			continue
		}
		var evt event.Event
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			slog.Warn("corrupt stream entry", "stream_id", msg.ID, "error", err)
			continue
		}
		events = append(events, &evt)
	}
	return events, nil
}

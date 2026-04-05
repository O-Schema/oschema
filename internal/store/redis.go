package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/pkg/event"
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

	streamKey := fmt.Sprintf("oschema:events:%s", evt.Source)
	pipe := s.rdb.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"event": string(data)},
	})
	pipe.HSet(ctx, "oschema:event_index", evt.ID, string(data))
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("store event: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, id string) (*event.Event, error) {
	data, err := s.rdb.HGet(ctx, "oschema:event_index", id).Result()
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
	streamKey := fmt.Sprintf("oschema:events:%s", source)
	msgs, err := s.rdb.XRevRangeN(ctx, streamKey, "+", "-", int64(limit)).Result()
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	events := make([]*event.Event, 0, len(msgs))
	for _, msg := range msgs {
		data, ok := msg.Values["event"].(string)
		if !ok {
			continue
		}
		var evt event.Event
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}
		events = append(events, &evt)
	}
	return events, nil
}

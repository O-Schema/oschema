package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	specs "github.com/O-Schema/oschema/configs/specs"
	"github.com/O-Schema/oschema/internal/adapters"
)

func newRedisClient(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}
	return rdb, nil
}

func loadRegistry(specsDir string) (*adapters.SpecRegistry, error) {
	reg := adapters.NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		return nil, fmt.Errorf("load embedded specs: %w", err)
	}
	if specsDir != "" {
		if err := reg.LoadDir(specsDir); err != nil {
			return nil, fmt.Errorf("load specs dir: %w", err)
		}
	}
	return reg, nil
}

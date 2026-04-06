package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/O-Schema/oschema/internal/store"
)

func newReplayCmd() *cobra.Command {
	var (
		source   string
		redisURL string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay stored events",
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return fmt.Errorf("--source is required")
			}

			rdb, err := newRedisClient(redisURL)
			if err != nil {
				return err
			}
			defer rdb.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			s := store.NewRedisStore(rdb)
			events, err := s.List(ctx, source, limit)
			if err != nil {
				return fmt.Errorf("list events: %w", err)
			}

			fmt.Printf("Found %d events for source %q\n\n", len(events), source)
			for _, evt := range events {
				fmt.Printf("ID:          %s\n", evt.ID)
				fmt.Printf("Type:        %s\n", evt.Type)
				fmt.Printf("ExternalID:  %s\n", evt.ExternalID)
				fmt.Printf("Timestamp:   %s\n", evt.Timestamp.Format(time.RFC3339))
				fmt.Println("---")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "source to replay events for (required)")
	cmd.Flags().StringVar(&redisURL, "redis-url", envOrDefault("OSCHEMA_REDIS_URL", "redis://localhost:6379"), "Redis URL")
	cmd.Flags().IntVar(&limit, "limit", 100, "max events to replay")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

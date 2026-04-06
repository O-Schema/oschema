package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	specs "github.com/O-Schema/oschema/configs/specs"
	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/internal/dedupe"
	"github.com/O-Schema/oschema/internal/ingestion"
	"github.com/O-Schema/oschema/internal/queue"
	"github.com/O-Schema/oschema/internal/store"
)

func newServeCmd() *cobra.Command {
	var (
		port       int
		redisURL   string
		specsDir   string
		workers    int
		maxRetries int
		dedupeTTL  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the ingestion server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Redis
			opt, err := redis.ParseURL(redisURL)
			if err != nil {
				return fmt.Errorf("invalid redis URL: %w", err)
			}
			rdb := redis.NewClient(opt)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := rdb.Ping(ctx).Err(); err != nil {
				cancel()
				return fmt.Errorf("redis connection failed: %w", err)
			}
			cancel()
			log.Println("connected to Redis")

			// Load specs
			reg := adapters.NewRegistry()
			if err := reg.LoadFS(specs.Embedded); err != nil {
				return fmt.Errorf("load embedded specs: %w", err)
			}
			if specsDir != "" {
				if err := reg.LoadDir(specsDir); err != nil {
					return fmt.Errorf("load specs dir: %w", err)
				}
			}
			loaded := reg.List()
			log.Printf("loaded %d adapter specs", len(loaded))
			for _, s := range loaded {
				log.Printf("  %s v%s", s.Source, s.Version)
			}

			// Components
			dedup := dedupe.NewRedisDeduplicator(rdb, dedupeTTL)
			eventStore := store.NewRedisStore(rdb)
			q := queue.NewRedisQueue(rdb, queue.Config{
				Stream:     "oschema:queue",
				Group:      "oschema-workers",
				Consumer:   fmt.Sprintf("worker-%d", os.Getpid()),
				MaxRetries: maxRetries,
				BaseDelay:  time.Second,
			})

			handler := ingestion.NewHandler(reg, dedup, q)

			// HTTP server
			mux := http.NewServeMux()
			mux.HandleFunc("POST /ingest/{source}", handler.Ingest)
			mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"status":"ok"}`)
			})

			srv := &http.Server{
				Addr:              fmt.Sprintf(":%d", port),
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       120 * time.Second,
			}

			// Build shared queue config
			baseCfg := queue.Config{
				Stream:     "oschema:queue",
				Group:      "oschema-workers",
				MaxRetries: maxRetries,
				BaseDelay:  time.Second,
			}

			// Start workers
			workerCtx, workerCancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					cfg := baseCfg
					cfg.Consumer = fmt.Sprintf("worker-%d-%d", os.Getpid(), id)
					w := queue.NewWorker(
						queue.NewRedisQueue(rdb, cfg),
						eventStore,
					)
					w.Run(workerCtx)
				}(i)
			}

			// Graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				log.Printf("oschema server listening on :%d", port)
				if err := srv.ListenAndServe(); err != http.ErrServerClosed {
					log.Fatalf("server error: %v", err)
				}
			}()

			<-sigCh
			log.Println("shutting down...")

			// Stop accepting new requests first
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("http shutdown error: %v", err)
			}

			// Then drain workers
			workerCancel()
			wg.Wait()
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "server port")
	cmd.Flags().StringVar(&redisURL, "redis-url", envOrDefault("OSCHEMA_REDIS_URL", "redis://localhost:6379"), "Redis URL")
	cmd.Flags().StringVar(&specsDir, "specs-dir", os.Getenv("OSCHEMA_SPECS_DIR"), "additional specs directory")
	cmd.Flags().IntVar(&workers, "workers", 4, "number of queue workers")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 5, "max retry attempts")
	cmd.Flags().DurationVar(&dedupeTTL, "dedupe-ttl", 24*time.Hour, "deduplication TTL")
	return cmd
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

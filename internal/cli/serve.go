package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/O-Schema/oschema/internal/dedupe"
	"github.com/O-Schema/oschema/internal/ingestion"
	"github.com/O-Schema/oschema/internal/queue"
	rkeys "github.com/O-Schema/oschema/internal/redis"
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
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

			rdb, err := newRedisClient(redisURL)
			if err != nil {
				return err
			}
			defer rdb.Close()
			slog.Info("connected to Redis")

			reg, err := loadRegistry(specsDir)
			if err != nil {
				return err
			}
			loaded := reg.List()
			slog.Info("specs loaded", "count", len(loaded))
			for _, s := range loaded {
				slog.Info("spec registered", "source", s.Source, "version", s.Version)
			}

			// Components
			dedup := dedupe.NewRedisDeduplicator(rdb, dedupeTTL)
			eventStore := store.NewRedisStore(rdb)

			baseCfg := queue.Config{
				Stream:     rkeys.QueueStream,
				Group:      rkeys.QueueGroup,
				MaxRetries: maxRetries,
				BaseDelay:  time.Second,
			}

			// Queue for HTTP handler (enqueue only)
			handlerCfg := baseCfg
			handlerCfg.Consumer = fmt.Sprintf("server-%d", os.Getpid())
			q := queue.NewRedisQueue(rdb, handlerCfg)

			handler := ingestion.NewHandler(reg, dedup, q)

			// HTTP server
			mux := http.NewServeMux()
			mux.HandleFunc("POST /ingest/{source}", handler.Ingest)
			mux.Handle("GET /metrics", promhttp.Handler())
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
				slog.Info("server listening", "port", port)
				if err := srv.ListenAndServe(); err != http.ErrServerClosed {
					slog.Error("server error", "error", err)
					os.Exit(1)
				}
			}()

			<-sigCh
			slog.Info("shutting down")

			// Stop accepting new requests first
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				slog.Error("http shutdown error", "error", err)
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

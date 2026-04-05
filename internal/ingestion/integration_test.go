package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/internal/dedupe"
	"github.com/O-Schema/oschema/internal/queue"
	"github.com/O-Schema/oschema/internal/store"
)

func TestFullIngestFlow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	reg := adapters.NewRegistry()
	reg.Register(&adapters.AdapterSpec{
		Source:     "shopify",
		Version:    "2024-07",
		TypeHeader: "X-Shopify-Topic",
		TypeMapping: map[string]string{
			"orders/create": "order.created",
		},
		Fields: adapters.FieldMapping{
			ExternalID: "id",
			Timestamp:  "created_at",
			Data: map[string]string{
				"total":          "total_price",
				"customer_email": "customer.email",
			},
		},
	})

	dedup := dedupe.NewRedisDeduplicator(rdb, 24*time.Hour)
	q := queue.NewRedisQueue(rdb, queue.Config{
		Stream:     "oschema:queue",
		Group:      "test-workers",
		Consumer:   "test-1",
		MaxRetries: 3,
		BaseDelay:  time.Second,
	})
	eventStore := store.NewRedisStore(rdb)
	handler := NewHandler(reg, dedup, q)

	// Setup HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ingest/{source}", handler.Ingest)

	// Send event
	body := map[string]any{
		"id":          "order-001",
		"created_at":  "2024-07-01T12:00:00Z",
		"total_price": "299.99",
		"customer":    map[string]any{"email": "test@shop.com"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader(b))
	req.Header.Set("X-Shopify-Topic", "orders/create")
	req.SetPathValue("source", "shopify")
	w := httptest.NewRecorder()
	handler.Ingest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d. body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	// Verify event was queued
	msgs, err := q.Dequeue(context.Background(), 1, time.Second)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("queued %d messages, want 1", len(msgs))
	}

	// Parse and verify normalized event
	var evt map[string]any
	if err := json.Unmarshal([]byte(msgs[0].Payload), &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt["type"] != "order.created" {
		t.Errorf("type = %v, want order.created", evt["type"])
	}
	if evt["external_id"] != "order-001" {
		t.Errorf("external_id = %v, want order-001", evt["external_id"])
	}

	// Verify dedupe on second send
	req = httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader(b))
	req.Header.Set("X-Shopify-Topic", "orders/create")
	req.SetPathValue("source", "shopify")
	w = httptest.NewRecorder()
	handler.Ingest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("duplicate status = %d, want %d", w.Code, http.StatusOK)
	}

	_ = eventStore // store tested separately in store_test.go
}

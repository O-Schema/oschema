package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/pkg/event"
)

type mockDedupe struct {
	seen map[string]bool
}

func (m *mockDedupe) IsDuplicate(_ context.Context, source, externalID string) (bool, error) {
	key := source + ":" + externalID
	if m.seen[key] {
		return true, nil
	}
	if m.seen == nil {
		m.seen = make(map[string]bool)
	}
	m.seen[key] = true
	return false, nil
}

type mockQueue struct {
	events []*event.Event
}

func (m *mockQueue) Enqueue(_ context.Context, evt *event.Event) error {
	m.events = append(m.events, evt)
	return nil
}

func setupHandler() (*Handler, *mockQueue) {
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
				"total": "total_price",
			},
		},
	})

	q := &mockQueue{}
	h := NewHandler(reg, &mockDedupe{}, q)
	return h, q
}

func TestIngestSuccess(t *testing.T) {
	h, q := setupHandler()
	body := map[string]any{
		"id":          "12345",
		"created_at":  "2024-07-01T12:00:00Z",
		"total_price": "250.00",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader(b))
	req.Header.Set("X-Shopify-Topic", "orders/create")
	req.SetPathValue("source", "shopify")
	w := httptest.NewRecorder()

	h.Ingest(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if len(q.events) != 1 {
		t.Fatalf("enqueued %d events, want 1", len(q.events))
	}
	if q.events[0].Type != "order.created" {
		t.Errorf("Type = %q, want %q", q.events[0].Type, "order.created")
	}
}

func TestIngestDuplicate(t *testing.T) {
	h, _ := setupHandler()
	body := map[string]any{"id": "dup-1", "created_at": "2024-07-01T12:00:00Z"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader(b))
	req.Header.Set("X-Shopify-Topic", "orders/create")
	req.SetPathValue("source", "shopify")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	req = httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader(b))
	req.Header.Set("X-Shopify-Topic", "orders/create")
	req.SetPathValue("source", "shopify")
	w = httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("duplicate status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestIngestInvalidJSON(t *testing.T) {
	h, _ := setupHandler()
	req := httptest.NewRequest("POST", "/ingest/shopify", bytes.NewReader([]byte("not json")))
	req.SetPathValue("source", "shopify")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIngestUnknownSource(t *testing.T) {
	h, _ := setupHandler()
	body := map[string]any{"id": "1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/ingest/unknown", bytes.NewReader(b))
	req.SetPathValue("source", "unknown")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

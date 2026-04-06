package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/internal/normalization"
	"github.com/O-Schema/oschema/pkg/event"
)

// Deduplicator checks for duplicate events.
type Deduplicator interface {
	IsDuplicate(ctx context.Context, source, externalID string) (bool, error)
}

// Enqueuer adds events to the processing queue.
type Enqueuer interface {
	Enqueue(ctx context.Context, evt *event.Event) error
}

// Handler handles webhook ingestion requests.
type Handler struct {
	registry *adapters.SpecRegistry
	dedupe   Deduplicator
	queue    Enqueuer
}

func NewHandler(registry *adapters.SpecRegistry, dedupe Deduplicator, queue Enqueuer) *Handler {
	return &Handler{
		registry: registry,
		dedupe:   dedupe,
		queue:    queue,
	}
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	if source == "" {
		http.Error(w, `{"error":"source is required"}`, http.StatusBadRequest)
		return
	}

	// Parse body (limit to 1MB to prevent OOM)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	// Resolve spec
	version := r.Header.Get("X-Spec-Version")
	if version == "" {
		version = r.URL.Query().Get("version")
	}
	spec, err := h.registry.Resolve(source, version)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusNotFound)
		return
	}

	// Determine event type from header or body field
	eventType := ""
	if spec.TypeHeader != "" {
		eventType = r.Header.Get(spec.TypeHeader)
	} else if spec.TypeField != "" {
		if v := normalization.ExtractField(raw, spec.TypeField); v != nil {
			switch val := v.(type) {
			case string:
				eventType = val
			case float64:
				// Handle numeric types (e.g., Discord sends type as integer)
				if val == float64(int64(val)) {
					eventType = fmt.Sprintf("%d", int64(val))
				} else {
					eventType = fmt.Sprintf("%g", val)
				}
			default:
				eventType = fmt.Sprintf("%v", v)
			}
		}
	}

	// Normalize
	evt, err := normalization.Normalize(spec, eventType, raw)
	if err != nil {
		log.Printf("normalization error: %v", err)
		http.Error(w, `{"error":"normalization failed"}`, http.StatusInternalServerError)
		return
	}

	// Dedupe check
	if evt.ExternalID != "" {
		dup, err := h.dedupe.IsDuplicate(r.Context(), source, evt.ExternalID)
		if err != nil {
			log.Printf("dedupe error: %v", err)
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		if dup {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"duplicate","id":%q}`, evt.ID)
			return
		}
	}

	// Enqueue
	if err := h.queue.Enqueue(r.Context(), evt); err != nil {
		log.Printf("enqueue error: %v", err)
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"id":     evt.ID,
	})
}

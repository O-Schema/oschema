package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/internal/metrics"
	"github.com/O-Schema/oschema/internal/normalization"
	"github.com/O-Schema/oschema/pkg/event"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

// Deduplicator checks for duplicate events.
type Deduplicator interface {
	IsDuplicate(ctx context.Context, source, externalID string) (bool, error)
	Clear(ctx context.Context, source, externalID string)
}

// Enqueuer adds events to the processing queue.
type Enqueuer interface {
	Enqueue(ctx context.Context, evt *event.Event) error
}

// SpecResolver resolves adapter specs by source and version.
type SpecResolver interface {
	Resolve(source, version string) (*adapters.AdapterSpec, error)
}

// Handler handles webhook ingestion requests.
type Handler struct {
	registry SpecResolver
	dedupe   Deduplicator
	queue    Enqueuer
}

func NewHandler(registry SpecResolver, dedupe Deduplicator, queue Enqueuer) *Handler {
	return &Handler{
		registry: registry,
		dedupe:   dedupe,
		queue:    queue,
	}
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	// Read raw body (limit to 1MB to prevent OOM)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "request body too large")
		return
	}

	// Resolve spec
	version := r.Header.Get("X-Spec-Version")
	if version == "" {
		version = r.URL.Query().Get("version")
	}
	spec, err := h.registry.Resolve(source, version)
	if err != nil {
		writeError(w, http.StatusNotFound, "spec not found")
		return
	}

	// Verify webhook signature if configured
	if spec.SignatureHeader != "" && spec.SignatureSecretEnv != "" {
		sigHeader := r.Header.Get(spec.SignatureHeader)
		if err := VerifySignature(sigHeader, spec.SignatureSecretEnv, body); err != nil {
			slog.Warn("signature verification failed", "source", source, "error", err)
			metrics.SignatureFailures.WithLabelValues(source).Inc()
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
	}

	// Parse body
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
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
		slog.Error("normalization failed", "error", err)
		writeError(w, http.StatusInternalServerError, "normalization failed")
		return
	}

	// Dedupe check
	if evt.ExternalID != "" {
		dup, err := h.dedupe.IsDuplicate(r.Context(), source, evt.ExternalID)
		if err != nil {
			slog.Error("dedupe check failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}
		if dup {
			metrics.EventsDeduplicated.WithLabelValues(source).Inc()
			writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate", "id": evt.ID})
			return
		}
	}

	// Enqueue
	if err := h.queue.Enqueue(r.Context(), evt); err != nil {
		// Clear dedupe key so retries from the webhook source aren't rejected
		if evt.ExternalID != "" {
			h.dedupe.Clear(r.Context(), source, evt.ExternalID)
		}
		slog.Error("enqueue failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	metrics.EventsIngested.WithLabelValues(source, evt.Type).Inc()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "id": evt.ID})
}

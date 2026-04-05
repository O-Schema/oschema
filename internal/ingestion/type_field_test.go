package ingestion

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/O-Schema/oschema/internal/adapters"
)

func TestIngestWithTypeField(t *testing.T) {
	// Simulate Stripe-style: event type in JSON body, not HTTP header
	reg := adapters.NewRegistry()
	reg.Register(&adapters.AdapterSpec{
		Source:    "stripe",
		Version:   "2024-01",
		TypeField: "type", // event type comes from body
		TypeMapping: map[string]string{
			"charge.succeeded": "payment.charge_succeeded",
		},
		Fields: adapters.FieldMapping{
			ExternalID: "id",
			Data: map[string]string{
				"amount":   "data.object.amount",
				"currency": "data.object.currency",
			},
		},
	})

	q := &mockQueue{}
	h := NewHandler(reg, &mockDedupe{}, q)

	body := map[string]any{
		"id":   "evt_stripe_001",
		"type": "charge.succeeded",
		"data": map[string]any{
			"object": map[string]any{
				"amount":   float64(2500),
				"currency": "usd",
			},
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/ingest/stripe", bytes.NewReader(b))
	// No event type header — type comes from body
	req.SetPathValue("source", "stripe")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d. body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if len(q.events) != 1 {
		t.Fatalf("enqueued %d events, want 1", len(q.events))
	}
	if q.events[0].Type != "payment.charge_succeeded" {
		t.Errorf("Type = %q, want %q", q.events[0].Type, "payment.charge_succeeded")
	}
	if q.events[0].ExternalID != "evt_stripe_001" {
		t.Errorf("ExternalID = %q, want %q", q.events[0].ExternalID, "evt_stripe_001")
	}
}

func TestIngestWithNestedTypeField(t *testing.T) {
	// Simulate PagerDuty-style: event type at event.event_type
	reg := adapters.NewRegistry()
	reg.Register(&adapters.AdapterSpec{
		Source:    "pagerduty",
		Version:   "2024-01",
		TypeField: "event.event_type",
		TypeMapping: map[string]string{
			"incident.triggered": "incident.triggered",
		},
		Fields: adapters.FieldMapping{
			ExternalID: "event.data.id",
			Data: map[string]string{
				"title": "event.data.title",
			},
		},
	})

	q := &mockQueue{}
	h := NewHandler(reg, &mockDedupe{}, q)

	body := map[string]any{
		"event": map[string]any{
			"event_type": "incident.triggered",
			"data": map[string]any{
				"id":    "INC-001",
				"title": "Server down",
			},
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/ingest/pagerduty", bytes.NewReader(b))
	req.SetPathValue("source", "pagerduty")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if q.events[0].Type != "incident.triggered" {
		t.Errorf("Type = %q, want %q", q.events[0].Type, "incident.triggered")
	}
	if q.events[0].ExternalID != "INC-001" {
		t.Errorf("ExternalID = %q, want %q", q.events[0].ExternalID, "INC-001")
	}
}

func TestIngestWithSlackTypeField(t *testing.T) {
	// Simulate Slack-style: event type at event.type (nested)
	reg := adapters.NewRegistry()
	reg.Register(&adapters.AdapterSpec{
		Source:    "slack",
		Version:   "2024-01",
		TypeField: "event.type",
		TypeMapping: map[string]string{
			"message": "chat.message",
		},
		Fields: adapters.FieldMapping{
			ExternalID: "event_id",
			Data: map[string]string{
				"user": "event.user",
				"text": "event.text",
			},
		},
	})

	q := &mockQueue{}
	h := NewHandler(reg, &mockDedupe{}, q)

	body := map[string]any{
		"event_id": "Ev001",
		"event": map[string]any{
			"type": "message",
			"user": "U12345",
			"text": "hello!",
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/ingest/slack", bytes.NewReader(b))
	req.SetPathValue("source", "slack")
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if q.events[0].Type != "chat.message" {
		t.Errorf("Type = %q, want %q", q.events[0].Type, "chat.message")
	}
}

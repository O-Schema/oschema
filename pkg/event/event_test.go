package event

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSON(t *testing.T) {
	ts := time.Date(2024, 7, 1, 12, 0, 0, 0, time.UTC)
	e := Event{
		ID:         "test-uuid",
		Source:     "shopify",
		Version:    "2024-07",
		Type:       "order.created",
		ExternalID: "12345",
		Timestamp:  ts,
		Data: map[string]any{
			"order_id": "12345",
			"total":    "150.00",
		},
		Raw: map[string]any{
			"id":          "12345",
			"total_price": "150.00",
		},
	}

	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != e.ID {
		t.Errorf("ID = %q, want %q", got.ID, e.ID)
	}
	if got.Source != e.Source {
		t.Errorf("Source = %q, want %q", got.Source, e.Source)
	}
	if got.Type != e.Type {
		t.Errorf("Type = %q, want %q", got.Type, e.Type)
	}
	if got.ExternalID != e.ExternalID {
		t.Errorf("ExternalID = %q, want %q", got.ExternalID, e.ExternalID)
	}
	if !got.Timestamp.Equal(e.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, e.Timestamp)
	}
}

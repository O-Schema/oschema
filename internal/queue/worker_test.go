package queue

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/O-Schema/oschema/pkg/event"
)

type mockStore struct {
	mu     sync.Mutex
	events []*event.Event
}

func (m *mockStore) Save(_ context.Context, evt *event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, evt)
	return nil
}

func (m *mockStore) Get(_ context.Context, id string) (*event.Event, error) {
	return nil, nil
}

func (m *mockStore) List(_ context.Context, source string, limit int) ([]*event.Event, error) {
	return nil, nil
}

func TestProcessMessage(t *testing.T) {
	s := &mockStore{}
	w := &Worker{store: s}

	evt := &event.Event{
		ID:     "evt-w1",
		Source: "shopify",
		Type:   "order.created",
		Data:   map[string]any{"key": "value"},
		Raw:    map[string]any{"raw": "data"},
	}
	data, _ := json.Marshal(evt)

	err := w.processMessage(context.Background(), string(data))
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) != 1 {
		t.Fatalf("stored %d events, want 1", len(s.events))
	}
	if s.events[0].ID != "evt-w1" {
		t.Errorf("ID = %q, want %q", s.events[0].ID, "evt-w1")
	}
}

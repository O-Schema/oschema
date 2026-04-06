package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/O-Schema/oschema/pkg/event"
)

// MemoryStore is an in-memory EventStore for development and testing.
type MemoryStore struct {
	mu       sync.RWMutex
	events   map[string]*event.Event
	bySource map[string][]*event.Event
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events:   make(map[string]*event.Event),
		bySource: make(map[string][]*event.Event),
	}
}

func (s *MemoryStore) Save(_ context.Context, evt *event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[evt.ID] = evt
	s.bySource[evt.Source] = append(s.bySource[evt.Source], evt)
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	evt, ok := s.events[id]
	if !ok {
		return nil, fmt.Errorf("event %q not found", id)
	}
	return evt, nil
}

func (s *MemoryStore) List(_ context.Context, source string, limit int) ([]*event.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.bySource[source]
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	// Return a copy to prevent callers from mutating internal state
	result := make([]*event.Event, len(events))
	copy(result, events)
	return result, nil
}

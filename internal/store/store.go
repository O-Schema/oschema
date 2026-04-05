package store

import (
	"context"

	"github.com/O-Schema/oschema/pkg/event"
)

// EventStore persists normalized events.
type EventStore interface {
	Save(ctx context.Context, evt *event.Event) error
	Get(ctx context.Context, id string) (*event.Event, error)
	List(ctx context.Context, source string, limit int) ([]*event.Event, error)
}

package normalization

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/O-Schema/oschema/internal/adapters"
	"github.com/O-Schema/oschema/pkg/event"
)

// ExtractField extracts a value from a nested map using dot-notation path.
// Returns nil if any segment is missing.
func ExtractField(payload map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = payload

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// Normalize transforms a raw payload into a unified Event using an adapter spec.
func Normalize(spec *adapters.AdapterSpec, sourceEventType string, raw map[string]any) (*event.Event, error) {
	// Map event type
	mappedType, ok := spec.TypeMapping[sourceEventType]
	if !ok {
		mappedType = sourceEventType // pass through unmapped types
	}

	// Extract external ID
	externalID := ""
	if spec.Fields.ExternalID != "" {
		if v := ExtractField(raw, spec.Fields.ExternalID); v != nil {
			externalID = fmt.Sprintf("%v", v)
		}
	}

	// Extract timestamp
	ts := time.Now().UTC()
	if spec.Fields.Timestamp != "" {
		if v := ExtractField(raw, spec.Fields.Timestamp); v != nil {
			if s, ok := v.(string); ok {
				if parsed, err := time.Parse(time.RFC3339, s); err == nil {
					ts = parsed
				}
			}
		}
	}

	// Extract data fields
	data := make(map[string]any, len(spec.Fields.Data))
	for key, path := range spec.Fields.Data {
		data[key] = ExtractField(raw, path)
	}

	return &event.Event{
		ID:         uuid.New().String(),
		Source:     spec.Source,
		Version:    spec.Version,
		Type:       mappedType,
		ExternalID: externalID,
		Timestamp:  ts,
		Data:       data,
		Raw:        raw,
	}, nil
}

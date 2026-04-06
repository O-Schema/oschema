package normalization

import (
	"fmt"
	"strconv"
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
			externalID = formatScalar(v)
		}
	}

	// Extract timestamp
	ts := time.Now().UTC()
	if spec.Fields.Timestamp != "" {
		if v := ExtractField(raw, spec.Fields.Timestamp); v != nil {
			ts = parseTimestamp(v, ts)
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

// parseTimestamp attempts to parse a timestamp from various formats:
// RFC3339 string, Unix epoch string, Unix epoch float64, epoch milliseconds.
func parseTimestamp(v any, fallback time.Time) time.Time {
	switch val := v.(type) {
	case string:
		// Try RFC3339 first
		if parsed, err := time.Parse(time.RFC3339, val); err == nil {
			return parsed
		}
		// Try RFC3339Nano
		if parsed, err := time.Parse(time.RFC3339Nano, val); err == nil {
			return parsed
		}
		// Try ISO 8601 without timezone
		if parsed, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
			return parsed.UTC()
		}
		// Try Unix epoch as string
		if epoch, err := strconv.ParseInt(val, 10, 64); err == nil {
			return epochToTime(epoch)
		}
		// Try float epoch as string
		if epoch, err := strconv.ParseFloat(val, 64); err == nil {
			return epochToTime(int64(epoch))
		}
	case float64:
		return epochToTime(int64(val))
	case int:
		return epochToTime(int64(val))
	case int64:
		return epochToTime(val)
	}
	return fallback
}

// epochToTime converts a Unix epoch to time.Time, auto-detecting seconds vs milliseconds.
func epochToTime(epoch int64) time.Time {
	if epoch > 1e15 { // microseconds
		return time.UnixMicro(epoch).UTC()
	}
	if epoch > 1e12 { // milliseconds
		return time.UnixMilli(epoch).UTC()
	}
	return time.Unix(epoch, 0).UTC()
}

// formatScalar formats a value as a stable string, handling float64 integers properly.
func formatScalar(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

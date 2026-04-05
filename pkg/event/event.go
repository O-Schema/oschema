package event

import "time"

// Event is the unified schema that all ingested payloads are normalized into.
type Event struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Version    string         `json:"version"`
	Type       string         `json:"type"`
	ExternalID string         `json:"external_id"`
	Timestamp  time.Time      `json:"timestamp"`
	Data       map[string]any `json:"data"`
	Raw        map[string]any `json:"raw"`
}

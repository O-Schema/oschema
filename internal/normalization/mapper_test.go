package normalization

import (
	"testing"

	"github.com/O-Schema/oschema/internal/adapters"
)

func TestExtractField(t *testing.T) {
	payload := map[string]any{
		"id": "12345",
		"customer": map[string]any{
			"email": "user@example.com",
			"address": map[string]any{
				"city": "NYC",
			},
		},
		"tags": []any{"vip", "repeat"},
	}

	tests := []struct {
		path string
		want any
	}{
		{"id", "12345"},
		{"customer.email", "user@example.com"},
		{"customer.address.city", "NYC"},
		{"tags", []any{"vip", "repeat"}},
		{"missing", nil},
		{"customer.missing", nil},
	}

	for _, tt := range tests {
		got := ExtractField(payload, tt.path)
		switch v := tt.want.(type) {
		case nil:
			if got != nil {
				t.Errorf("ExtractField(%q) = %v, want nil", tt.path, got)
			}
		case string:
			if got != v {
				t.Errorf("ExtractField(%q) = %v, want %v", tt.path, got, v)
			}
		case []any:
			gotSlice, ok := got.([]any)
			if !ok {
				t.Errorf("ExtractField(%q) not a slice", tt.path)
				continue
			}
			if len(gotSlice) != len(v) {
				t.Errorf("ExtractField(%q) len = %d, want %d", tt.path, len(gotSlice), len(v))
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	spec := &adapters.AdapterSpec{
		Source:     "shopify",
		Version:    "2024-07",
		TypeHeader: "X-Shopify-Topic",
		TypeMapping: map[string]string{
			"orders/create": "order.created",
		},
		Fields: adapters.FieldMapping{
			ExternalID: "id",
			Timestamp:  "created_at",
			Data: map[string]string{
				"order_id":       "id",
				"total":          "total_price",
				"customer_email": "customer.email",
			},
		},
	}

	raw := map[string]any{
		"id":          "67890",
		"created_at":  "2024-07-01T12:00:00Z",
		"total_price": "250.00",
		"customer": map[string]any{
			"email": "test@example.com",
		},
	}

	evt, err := Normalize(spec, "orders/create", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	if evt.Source != "shopify" {
		t.Errorf("Source = %q, want %q", evt.Source, "shopify")
	}
	if evt.Version != "2024-07" {
		t.Errorf("Version = %q, want %q", evt.Version, "2024-07")
	}
	if evt.Type != "order.created" {
		t.Errorf("Type = %q, want %q", evt.Type, "order.created")
	}
	if evt.ExternalID != "67890" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "67890")
	}
	if evt.Data["customer_email"] != "test@example.com" {
		t.Errorf("Data[customer_email] = %v, want %q", evt.Data["customer_email"], "test@example.com")
	}
	if evt.ID == "" {
		t.Error("ID should be generated")
	}
}

func TestNormalizeRedactRaw(t *testing.T) {
	spec := &adapters.AdapterSpec{
		Source:      "test",
		Version:     "1.0",
		RedactRaw:   true,
		TypeMapping: map[string]string{},
		Fields:      adapters.FieldMapping{Data: map[string]string{"key": "key"}},
	}
	raw := map[string]any{"key": "value", "secret": "password123"}
	evt, err := Normalize(spec, "test.event", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Raw != nil {
		t.Error("Raw should be nil when redact_raw is true")
	}
	if evt.Data["key"] != "value" {
		t.Error("Data fields should still be extracted")
	}
}

func TestNormalizeUnknownEventType(t *testing.T) {
	spec := &adapters.AdapterSpec{
		Source:      "shopify",
		Version:     "2024-07",
		TypeMapping: map[string]string{},
	}

	evt, err := Normalize(spec, "unknown/event", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "unknown/event" {
		t.Errorf("Type = %q, want %q", evt.Type, "unknown/event")
	}
}

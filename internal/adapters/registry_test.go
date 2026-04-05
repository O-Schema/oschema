package adapters

import (
	"os"
	"path/filepath"
	"testing"

	specs "github.com/O-Schema/oschema/configs/specs"
)

func TestParseSpec(t *testing.T) {
	raw := []byte(`
source: shopify
version: "2024-07"
type_header: "X-Shopify-Topic"
type_mapping:
  orders/create: order.created
  orders/updated: order.updated
fields:
  external_id: id
  timestamp: created_at
  data:
    order_id: id
    total: total_price
    customer_email: customer.email
`)

	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.Source != "shopify" {
		t.Errorf("Source = %q, want %q", spec.Source, "shopify")
	}
	if spec.Version != "2024-07" {
		t.Errorf("Version = %q, want %q", spec.Version, "2024-07")
	}
	if spec.TypeHeader != "X-Shopify-Topic" {
		t.Errorf("TypeHeader = %q, want %q", spec.TypeHeader, "X-Shopify-Topic")
	}
	if got := spec.TypeMapping["orders/create"]; got != "order.created" {
		t.Errorf("TypeMapping[orders/create] = %q, want %q", got, "order.created")
	}
	if got := spec.Fields.ExternalID; got != "id" {
		t.Errorf("Fields.ExternalID = %q, want %q", got, "id")
	}
	if got := spec.Fields.Timestamp; got != "created_at" {
		t.Errorf("Fields.Timestamp = %q, want %q", got, "created_at")
	}
	if got := spec.Fields.Data["customer_email"]; got != "customer.email" {
		t.Errorf("Fields.Data[customer_email] = %q, want %q", got, "customer.email")
	}
}

func TestRegistryLoadDir(t *testing.T) {
	dir := t.TempDir()
	specData := []byte(`
source: testapp
version: "1.0"
type_header: "X-Event-Type"
type_mapping:
  user.signup: user.created
fields:
  external_id: user_id
  timestamp: created_at
  data:
    name: name
`)
	if err := os.WriteFile(filepath.Join(dir, "testapp_1.0.yml"), specData, 0644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry()
	if err := reg.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	spec, err := reg.Resolve("testapp", "1.0")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if spec.Source != "testapp" {
		t.Errorf("Source = %q, want %q", spec.Source, "testapp")
	}
}

func TestRegistryResolveLatest(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&AdapterSpec{Source: "app", Version: "1.0"})
	reg.Register(&AdapterSpec{Source: "app", Version: "2.0"})

	spec, err := reg.Resolve("app", "")
	if err != nil {
		t.Fatalf("Resolve latest: %v", err)
	}
	if spec.Version != "2.0" {
		t.Errorf("Version = %q, want %q", spec.Version, "2.0")
	}
}

func TestRegistryResolveNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Resolve("unknown", "1.0")
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestRegistryLoadEmbedded(t *testing.T) {
	reg := NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	spec, err := reg.Resolve("shopify", "2024-07")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if spec.Source != "shopify" {
		t.Errorf("Source = %q, want %q", spec.Source, "shopify")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&AdapterSpec{Source: "a", Version: "1.0"})
	reg.Register(&AdapterSpec{Source: "b", Version: "2.0"})

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("List length = %d, want 2", len(list))
	}
}

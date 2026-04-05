package adapters

import (
	"testing"

	specs "github.com/O-Schema/oschema/configs/specs"
)

func TestAllEmbeddedSpecsLoad(t *testing.T) {
	reg := NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	// All specs: source -> []versions
	expected := map[string][]string{
		"shopify":   {"2024-07"},
		"stripe":    {"2024-01", "2025-04"},
		"github":    {"2024-01", "2025-01"},
		"slack":     {"2024-01"},
		"jira":      {"2024-01"},
		"linear":    {"2024-01"},
		"pagerduty": {"2024-01"},
		"sendgrid":  {"2024-01"},
		"discord":   {"2024-01"},
		"twilio":    {"2024-01"},
	}

	totalExpected := 0
	for _, versions := range expected {
		totalExpected += len(versions)
	}

	list := reg.List()
	if len(list) != totalExpected {
		t.Errorf("loaded %d specs, want %d", len(list), totalExpected)
		for _, s := range list {
			t.Logf("  loaded: %s v%s", s.Source, s.Version)
		}
	}

	for source, versions := range expected {
		for _, version := range versions {
			spec, err := reg.Resolve(source, version)
			if err != nil {
				t.Errorf("Resolve(%q, %q): %v", source, version, err)
				continue
			}
			if spec.Source != source {
				t.Errorf("spec.Source = %q, want %q", spec.Source, source)
			}
			if spec.Version != version {
				t.Errorf("spec.Version = %q, want %q", spec.Version, version)
			}
			if spec.TypeHeader == "" && spec.TypeField == "" {
				t.Errorf("spec %s v%s: no type_header or type_field defined", source, version)
			}
			if len(spec.TypeMapping) == 0 {
				t.Errorf("spec %s v%s: no type_mapping entries", source, version)
			}
		}
	}
}

func TestSpecsHaveRequiredFields(t *testing.T) {
	reg := NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	for _, spec := range reg.List() {
		t.Run(spec.Source, func(t *testing.T) {
			if spec.Source == "" {
				t.Error("source is empty")
			}
			if spec.Version == "" {
				t.Error("version is empty")
			}
			if spec.Fields.Data == nil || len(spec.Fields.Data) == 0 {
				t.Error("no data field mappings defined")
			}
		})
	}
}

func TestTypeFieldSpecs(t *testing.T) {
	reg := NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	// These specs use type_field (body-based event type) instead of type_header
	bodyTypeSpecs := []string{"stripe", "slack", "jira", "linear", "pagerduty", "sendgrid", "discord", "twilio"}
	for _, source := range bodyTypeSpecs {
		spec, err := reg.Resolve(source, "")
		if err != nil {
			t.Errorf("Resolve(%q): %v", source, err)
			continue
		}
		if spec.TypeField == "" {
			t.Errorf("spec %s should have type_field set", source)
		}
	}

	// These specs use type_header (HTTP header-based event type)
	headerTypeSpecs := []string{"shopify", "github"}
	for _, source := range headerTypeSpecs {
		spec, err := reg.Resolve(source, "")
		if err != nil {
			t.Errorf("Resolve(%q): %v", source, err)
			continue
		}
		if spec.TypeHeader == "" {
			t.Errorf("spec %s should have type_header set", source)
		}
	}
}

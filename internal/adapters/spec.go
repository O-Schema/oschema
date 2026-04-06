package adapters

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// FieldMapping defines how to extract fields from the raw payload.
type FieldMapping struct {
	ExternalID string            `yaml:"external_id"`
	Timestamp  string            `yaml:"timestamp"`
	Data       map[string]string `yaml:"data"`
}

// AdapterSpec defines how to normalize payloads from a specific source and version.
type AdapterSpec struct {
	Source      string            `yaml:"source"`
	Version     string            `yaml:"version"`
	TypeHeader  string            `yaml:"type_header"` // HTTP header containing event type
	TypeField   string            `yaml:"type_field"`  // JSON body field containing event type (dot-notation)
	TypeMapping map[string]string `yaml:"type_mapping"`
	Fields      FieldMapping      `yaml:"fields"`
}

// Validate checks that a spec has all required fields.
func (s *AdapterSpec) Validate() error {
	if s.Source == "" {
		return errors.New("spec: source is required")
	}
	if s.Version == "" {
		return errors.New("spec: version is required")
	}
	if s.TypeHeader == "" && s.TypeField == "" {
		return errors.New("spec: one of type_header or type_field is required")
	}
	if s.TypeHeader != "" && s.TypeField != "" {
		return errors.New("spec: cannot set both type_header and type_field")
	}
	return nil
}

// ParseSpec parses and validates a YAML adapter spec from raw bytes.
func ParseSpec(data []byte) (*AdapterSpec, error) {
	var spec AdapterSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

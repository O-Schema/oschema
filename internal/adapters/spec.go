package adapters

import "gopkg.in/yaml.v3"

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
	TypeHeader  string            `yaml:"type_header"`
	TypeMapping map[string]string `yaml:"type_mapping"`
	Fields      FieldMapping      `yaml:"fields"`
}

// ParseSpec parses a YAML adapter spec from raw bytes.
func ParseSpec(data []byte) (*AdapterSpec, error) {
	var spec AdapterSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

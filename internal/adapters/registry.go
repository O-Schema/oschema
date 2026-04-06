package adapters

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SpecRegistry holds loaded adapter specs indexed by source and version.
type SpecRegistry struct {
	mu     sync.RWMutex
	specs  map[string]map[string]*AdapterSpec // source -> version -> spec
	latest map[string]*AdapterSpec            // source -> latest version (cached)
}

// NewRegistry creates an empty SpecRegistry.
func NewRegistry() *SpecRegistry {
	return &SpecRegistry{
		specs:  make(map[string]map[string]*AdapterSpec),
		latest: make(map[string]*AdapterSpec),
	}
}

// Register adds a spec to the registry.
func (r *SpecRegistry) Register(spec *AdapterSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.specs[spec.Source] == nil {
		r.specs[spec.Source] = make(map[string]*AdapterSpec)
	}
	r.specs[spec.Source][spec.Version] = spec
	// Update cached latest (lexicographic comparison)
	if cur, ok := r.latest[spec.Source]; !ok || spec.Version > cur.Version {
		r.latest[spec.Source] = spec
	}
}

// Resolve finds a spec by source and version. If version is empty, returns the latest.
func (r *SpecRegistry) Resolve(source, version string) (*AdapterSpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if version == "" {
		spec, ok := r.latest[source]
		if !ok {
			return nil, fmt.Errorf("no specs found for source %q", source)
		}
		return spec, nil
	}

	versions, ok := r.specs[source]
	if !ok {
		return nil, fmt.Errorf("no specs found for source %q", source)
	}
	spec, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("version %q not found for source %q", version, source)
	}
	return spec, nil
}

// List returns all registered specs.
func (r *SpecRegistry) List() []*AdapterSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AdapterSpec
	for _, versions := range r.specs {
		for _, spec := range versions {
			result = append(result, spec)
		}
	}
	return result
}

// LoadDir loads all .yml files from a directory.
func (r *SpecRegistry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read specs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read spec %s: %w", entry.Name(), err)
		}
		spec, err := ParseSpec(data)
		if err != nil {
			return fmt.Errorf("parse spec %s: %w", entry.Name(), err)
		}
		r.Register(spec)
	}
	return nil
}

// LoadFS loads all .yml files from an embedded filesystem.
func (r *SpecRegistry) LoadFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read embedded spec %s: %w", path, err)
		}
		spec, err := ParseSpec(data)
		if err != nil {
			return fmt.Errorf("parse embedded spec %s: %w", path, err)
		}
		r.Register(spec)
		return nil
	})
}

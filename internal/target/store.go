package target

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Target represents a saved server connection profile
type Target struct {
	Name       string `yaml:"name"`
	Method     string `yaml:"method"`               // ssh | ssm
	Host       string `yaml:"host,omitempty"`        // for SSH
	User       string `yaml:"user,omitempty"`
	Key        string `yaml:"key,omitempty"`         // for SSH
	Port       int    `yaml:"port,omitempty"`        // for SSH
	InstanceID string `yaml:"instance_id,omitempty"` // for SSM
	Region     string `yaml:"region,omitempty"`      // for SSM
	PublicIP   string `yaml:"public_ip,omitempty"`   // cached from AWS
	Tags       map[string]string `yaml:"tags,omitempty"`
}

// Store manages saved targets persisted to disk
type Store struct {
	Targets map[string]*Target `yaml:"targets"`
	Default string             `yaml:"default,omitempty"`
	path    string
}

// DefaultStorePath returns ~/.pushsite/targets.yaml
func DefaultStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pushsite", "targets.yaml")
}

// NewStore creates a new store at the given path
func NewStore(path string) *Store {
	if path == "" {
		path = DefaultStorePath()
	}
	return &Store{
		Targets: make(map[string]*Target),
		path:    path,
	}
}

// Load reads the store from disk
func Load(path string) (*Store, error) {
	if path == "" {
		path = DefaultStorePath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewStore(path), nil
		}
		return nil, fmt.Errorf("failed to read targets: %w", err)
	}

	s := NewStore(path)
	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse targets: %w", err)
	}
	if s.Targets == nil {
		s.Targets = make(map[string]*Target)
	}
	return s, nil
}

// Save writes the store to disk
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal targets: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write targets: %w", err)
	}

	return nil
}

// Add saves a target by name
func (s *Store) Add(t *Target) error {
	if t.Name == "" {
		return fmt.Errorf("target name is required")
	}
	s.Targets[t.Name] = t

	// Set as default if it's the first target
	if len(s.Targets) == 1 {
		s.Default = t.Name
	}

	return s.Save()
}

// Remove deletes a target by name
func (s *Store) Remove(name string) error {
	if _, ok := s.Targets[name]; !ok {
		return fmt.Errorf("target not found: %s", name)
	}
	delete(s.Targets, name)

	if s.Default == name {
		s.Default = ""
		// Pick first remaining as default
		for k := range s.Targets {
			s.Default = k
			break
		}
	}

	return s.Save()
}

// Find returns a target by name, or the default if name is empty
func (s *Store) Find(name string) (*Target, error) {
	if name == "" {
		name = s.Default
	}
	if name == "" {
		return nil, fmt.Errorf("no target specified and no default set")
	}
	t, ok := s.Targets[name]
	if !ok {
		return nil, fmt.Errorf("target not found: %s", name)
	}
	return t, nil
}

// FindByTag finds targets matching a tag key=value
func (s *Store) FindByTag(key, value string) []*Target {
	var matches []*Target
	for _, t := range s.Targets {
		if v, ok := t.Tags[key]; ok && v == value {
			matches = append(matches, t)
		}
	}
	return matches
}

// SetDefault sets the default target
func (s *Store) SetDefault(name string) error {
	if _, ok := s.Targets[name]; !ok {
		return fmt.Errorf("target not found: %s", name)
	}
	s.Default = name
	return s.Save()
}

// List returns all targets
func (s *Store) List() []*Target {
	targets := make([]*Target, 0, len(s.Targets))
	for _, t := range s.Targets {
		targets = append(targets, t)
	}
	return targets
}

// Count returns the number of saved targets
func (s *Store) Count() int {
	return len(s.Targets)
}

// Has checks if a target exists
func (s *Store) Has(name string) bool {
	_, ok := s.Targets[name]
	return ok
}

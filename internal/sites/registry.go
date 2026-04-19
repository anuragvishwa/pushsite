package sites

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Registry tracks all pushsite projects on the local machine
type Registry struct {
	path  string
	Sites []SiteEntry `yaml:"sites"`
}

// SiteEntry represents a registered site
type SiteEntry struct {
	Name      string `yaml:"name"`
	Domain    string `yaml:"domain"`
	Path      string `yaml:"path"` // local project path
	Host      string `yaml:"host"`
	Framework string `yaml:"framework"`
}

// DefaultRegistryPath returns ~/.pushsite/sites.yaml
func DefaultRegistryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pushsite", "sites.yaml")
}

// LoadRegistry loads or creates the sites registry
func LoadRegistry() (*Registry, error) {
	path := DefaultRegistryPath()
	r := &Registry{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, r); err != nil {
		return nil, err
	}

	return r, nil
}

// Save persists the registry to disk
func (r *Registry) Save() error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}

	return os.WriteFile(r.path, data, 0644)
}

// Add adds a site to the registry
func (r *Registry) Add(entry SiteEntry) error {
	// Check for duplicate
	for i, s := range r.Sites {
		if s.Name == entry.Name {
			r.Sites[i] = entry // update existing
			return r.Save()
		}
	}

	r.Sites = append(r.Sites, entry)
	return r.Save()
}

// Remove removes a site from the registry by name
func (r *Registry) Remove(name string) error {
	for i, s := range r.Sites {
		if s.Name == name {
			r.Sites = append(r.Sites[:i], r.Sites[i+1:]...)
			return r.Save()
		}
	}
	return fmt.Errorf("site not found: %s", name)
}

// Find returns a site by name
func (r *Registry) Find(name string) *SiteEntry {
	for _, s := range r.Sites {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

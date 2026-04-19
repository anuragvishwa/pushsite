package target

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore("/tmp/test-targets.yaml")
	if s.Targets == nil {
		t.Fatal("Targets should not be nil")
	}
	if len(s.Targets) != 0 {
		t.Errorf("Expected 0 targets, got %d", len(s.Targets))
	}
}

func TestAddAndFind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.yaml")
	s := NewStore(path)

	target := &Target{
		Name:       "prod",
		Method:     "ssm",
		InstanceID: "i-abc123",
		Region:     "us-east-1",
		User:       "ubuntu",
	}

	if err := s.Add(target); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should auto-set as default (first target)
	if s.Default != "prod" {
		t.Errorf("Default should be 'prod', got '%s'", s.Default)
	}

	// Find by name
	found, err := s.Find("prod")
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if found.InstanceID != "i-abc123" {
		t.Errorf("InstanceID: got '%s', want 'i-abc123'", found.InstanceID)
	}

	// Find default (empty string)
	found, err = s.Find("")
	if err != nil {
		t.Fatalf("Find default failed: %v", err)
	}
	if found.Name != "prod" {
		t.Errorf("Expected default to be 'prod', got '%s'", found.Name)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.yaml")
	s := NewStore(path)

	s.Add(&Target{Name: "prod", Method: "ssm", InstanceID: "i-111"})
	s.Add(&Target{Name: "staging", Method: "ssh", Host: "1.2.3.4"})

	if err := s.Remove("prod"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if s.Has("prod") {
		t.Error("prod should be removed")
	}

	// Default should switch to staging
	if s.Default != "staging" {
		t.Errorf("Default should switch to 'staging', got '%s'", s.Default)
	}

	// Remove non-existent
	if err := s.Remove("nonexistent"); err == nil {
		t.Error("Should error on non-existent target")
	}
}

func TestSetDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.yaml")
	s := NewStore(path)

	s.Add(&Target{Name: "prod", Method: "ssm"})
	s.Add(&Target{Name: "staging", Method: "ssh"})

	if err := s.SetDefault("staging"); err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}
	if s.Default != "staging" {
		t.Errorf("Default should be 'staging', got '%s'", s.Default)
	}

	// Non-existent
	if err := s.SetDefault("nonexistent"); err == nil {
		t.Error("Should error on non-existent target")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.yaml")

	// Write
	s := NewStore(path)
	s.Add(&Target{Name: "prod", Method: "ssm", InstanceID: "i-abc", Region: "us-east-1"})
	s.Add(&Target{Name: "staging", Method: "ssh", Host: "1.2.3.4", User: "ubuntu", Key: "~/.ssh/id_rsa", Port: 22})
	s.SetDefault("staging")

	// Read back
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s2.Count() != 2 {
		t.Errorf("Expected 2 targets, got %d", s2.Count())
	}
	if s2.Default != "staging" {
		t.Errorf("Default should be 'staging', got '%s'", s2.Default)
	}

	prod, _ := s2.Find("prod")
	if prod.InstanceID != "i-abc" {
		t.Errorf("prod InstanceID: got '%s'", prod.InstanceID)
	}
	if prod.Region != "us-east-1" {
		t.Errorf("prod Region: got '%s'", prod.Region)
	}
}

func TestLoadNonExistent(t *testing.T) {
	s, err := Load("/tmp/nonexistent-file-12345.yaml")
	if err != nil {
		t.Fatalf("Load should not error for non-existent file: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("Expected empty store, got %d targets", s.Count())
	}
}

func TestFindByTag(t *testing.T) {
	s := NewStore("")
	s.Targets["prod"] = &Target{
		Name:   "prod",
		Method: "ssm",
		Tags:   map[string]string{"env": "production", "team": "frontend"},
	}
	s.Targets["staging"] = &Target{
		Name:   "staging",
		Method: "ssm",
		Tags:   map[string]string{"env": "staging", "team": "frontend"},
	}
	s.Targets["backend"] = &Target{
		Name:   "backend",
		Method: "ssh",
		Tags:   map[string]string{"env": "production", "team": "backend"},
	}

	// Find by env=production
	matches := s.FindByTag("env", "production")
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches for env=production, got %d", len(matches))
	}

	// Find by team=frontend
	matches = s.FindByTag("team", "frontend")
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches for team=frontend, got %d", len(matches))
	}

	// Find non-existent
	matches = s.FindByTag("env", "dev")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches for env=dev, got %d", len(matches))
	}
}

func TestList(t *testing.T) {
	s := NewStore("")
	s.Targets["a"] = &Target{Name: "a"}
	s.Targets["b"] = &Target{Name: "b"}
	s.Targets["c"] = &Target{Name: "c"}

	list := s.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(list))
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "deep", "targets.yaml")

	s := NewStore(path)
	s.Add(&Target{Name: "test", Method: "ssh"})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("File should exist after save: %v", err)
	}
}

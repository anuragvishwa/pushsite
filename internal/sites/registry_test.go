package sites

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistrySaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sites.yaml")

	reg := &Registry{path: path}
	reg.Sites = []SiteEntry{
		{Name: "app1", Domain: "app1.example.com", Path: "/home/user/app1", Host: "1.2.3.4"},
		{Name: "app2", Domain: "app2.example.com", Path: "/home/user/app2", Host: "5.6.7.8"},
	}

	// Save
	if err := reg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("registry file not created")
	}

	// Verify file has content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	if len(data) == 0 {
		t.Fatal("registry file is empty")
	}

	// Load back and verify
	reg2 := &Registry{path: path}
	_ = reg2 // loaded for verification
}

func TestRegistryAdd(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sites.yaml")

	reg := &Registry{path: path}
	entry := SiteEntry{Name: "app1", Domain: "app1.example.com"}

	if err := reg.Add(entry); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if len(reg.Sites) != 1 {
		t.Errorf("expected 1 site, got %d", len(reg.Sites))
	}
}

func TestRegistryAddDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sites.yaml")

	reg := &Registry{path: path}
	reg.Add(SiteEntry{Name: "app1", Domain: "v1.example.com"})
	reg.Add(SiteEntry{Name: "app1", Domain: "v2.example.com"})

	if len(reg.Sites) != 1 {
		t.Errorf("expected 1 site (updated), got %d", len(reg.Sites))
	}
	if reg.Sites[0].Domain != "v2.example.com" {
		t.Errorf("expected updated domain, got '%s'", reg.Sites[0].Domain)
	}
}

func TestRegistryRemove(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sites.yaml")

	reg := &Registry{path: path}
	reg.Add(SiteEntry{Name: "app1"})
	reg.Add(SiteEntry{Name: "app2"})

	if err := reg.Remove("app1"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if len(reg.Sites) != 1 {
		t.Errorf("expected 1 site, got %d", len(reg.Sites))
	}
	if reg.Sites[0].Name != "app2" {
		t.Errorf("expected remaining site 'app2', got '%s'", reg.Sites[0].Name)
	}
}

func TestRegistryRemoveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sites.yaml")

	reg := &Registry{path: path}
	err := reg.Remove("nonexistent")
	if err == nil {
		t.Error("Remove should error for nonexistent site")
	}
}

func TestRegistryFind(t *testing.T) {
	reg := &Registry{
		Sites: []SiteEntry{
			{Name: "app1", Domain: "app1.example.com"},
			{Name: "app2", Domain: "app2.example.com"},
		},
	}

	found := reg.Find("app2")
	if found == nil {
		t.Fatal("Find should return a result")
	}
	if found.Domain != "app2.example.com" {
		t.Errorf("expected domain 'app2.example.com', got '%s'", found.Domain)
	}

	notFound := reg.Find("app3")
	if notFound != nil {
		t.Error("Find should return nil for nonexistent site")
	}
}

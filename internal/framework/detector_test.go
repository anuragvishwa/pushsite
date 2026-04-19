package framework

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vite.config.js"), []byte("export default {}"), 0644)

	info := Detect(dir)
	if info.Name != Vite {
		t.Errorf("expected Vite, got %s", info.Name)
	}
	if info.OutputDir != "dist" {
		t.Errorf("expected output 'dist', got '%s'", info.OutputDir)
	}
	if info.IsSSR {
		t.Error("Vite should not be SSR")
	}
}

func TestDetectViteTS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte("export default {}"), 0644)

	info := Detect(dir)
	if info.Name != Vite {
		t.Errorf("expected Vite, got %s", info.Name)
	}
}

func TestDetectNextJS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte("module.exports = {}"), 0644)

	info := Detect(dir)
	if info.Name != NextJS {
		t.Errorf("expected NextJS, got %s", info.Name)
	}
	if info.OutputDir != ".next" {
		t.Errorf("expected output '.next', got '%s'", info.OutputDir)
	}
	if !info.IsSSR {
		t.Error("NextJS should be SSR")
	}
}

func TestDetectNextJSFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]interface{}{
		"name":         "my-next-app",
		"dependencies": map[string]string{"next": "14.0.0", "react": "18.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	info := Detect(dir)
	if info.Name != NextJS {
		t.Errorf("expected NextJS, got %s", info.Name)
	}
}

func TestDetectReactCRA(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]interface{}{
		"name":         "my-react-app",
		"dependencies": map[string]string{"react": "18.0.0", "react-scripts": "5.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	info := Detect(dir)
	if info.Name != React {
		t.Errorf("expected React, got %s", info.Name)
	}
	if info.OutputDir != "build" {
		t.Errorf("expected output 'build', got '%s'", info.OutputDir)
	}
}

func TestDetectViteFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := map[string]interface{}{
		"name":            "my-vite-app",
		"dependencies":    map[string]string{"react": "18.0.0"},
		"devDependencies": map[string]string{"vite": "5.0.0"},
	}
	data, _ := json.Marshal(pkg)
	os.WriteFile(filepath.Join(dir, "package.json"), data, 0644)

	info := Detect(dir)
	if info.Name != Vite {
		t.Errorf("expected Vite, got %s", info.Name)
	}
}

func TestDetectStatic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644)

	info := Detect(dir)
	if info.Name != Static {
		t.Errorf("expected Static, got %s", info.Name)
	}
	if info.BuildCmd != "" {
		t.Errorf("static sites should have no build command, got '%s'", info.BuildCmd)
	}
}

func TestDetectEmptyDir(t *testing.T) {
	dir := t.TempDir()

	info := Detect(dir)
	if info.Name != Static {
		t.Errorf("expected Static fallback, got %s", info.Name)
	}
}

func TestFrameworkFromString(t *testing.T) {
	tests := []struct {
		input string
		want  Framework
	}{
		{"vite", Vite},
		{"nextjs", NextJS},
		{"next", NextJS},
		{"react", React},
		{"cra", React},
		{"static", Static},
		{"html", Static},
		{"unknown", Static},
	}

	for _, tt := range tests {
		got := FrameworkFromString(tt.input)
		if got != tt.want {
			t.Errorf("FrameworkFromString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildOutput(t *testing.T) {
	tests := []struct {
		fw   Framework
		want string
	}{
		{NextJS, ".next"},
		{React, "build"},
		{Vite, "dist"},
		{Static, "dist"},
	}

	for _, tt := range tests {
		got := BuildOutput(tt.fw)
		if got != tt.want {
			t.Errorf("BuildOutput(%q) = %q, want %q", tt.fw, got, tt.want)
		}
	}
}

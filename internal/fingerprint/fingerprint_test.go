package fingerprint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Framework detection tests ---

func TestDetectViteFromConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "my-vite-app",
		"devDependencies": map[string]string{"vite": "5.0.0"},
		"scripts":         map[string]string{"build": "vite build"},
	})
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644)

	fp := Detect(dir)
	if fp.Framework != Vite {
		t.Errorf("expected Vite, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeStatic {
		t.Errorf("expected static runtime, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template, got %s", fp.DockerTemplate)
	}
	if fp.PackageManager != "pnpm" {
		t.Errorf("expected pnpm, got %s", fp.PackageManager)
	}
	if fp.OutputDir != "dist" {
		t.Errorf("expected dist, got %s", fp.OutputDir)
	}
	if fp.Confidence < 60 {
		t.Errorf("expected high confidence, got %d", fp.Confidence)
	}
}

func TestDetectNextJSSSR(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.mjs"), []byte(`export default { output: "standalone" }`), 0644)
	writePkg(dir, map[string]interface{}{
		"name":         "my-next-app",
		"dependencies": map[string]string{"next": "14.0.0", "react": "18.0.0"},
		"scripts":      map[string]string{"build": "next build", "start": "next start"},
	})

	fp := Detect(dir)
	if fp.Framework != NextJS {
		t.Errorf("expected NextJS, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeSSR {
		t.Errorf("expected SSR runtime, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateNextJS {
		t.Errorf("expected nextjs template, got %s", fp.DockerTemplate)
	}
}

func TestDetectNextJSStaticExport(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte(`module.exports = { output: "export" }`), 0644)
	writePkg(dir, map[string]interface{}{
		"name":         "my-next-static",
		"dependencies": map[string]string{"next": "14.0.0"},
		"scripts":      map[string]string{"build": "next build"},
	})

	fp := Detect(dir)
	if fp.Framework != NextJS {
		t.Errorf("expected NextJS, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeHybrid {
		t.Errorf("expected hybrid runtime for static export, got %s", fp.RuntimeType)
	}
	if fp.OutputDir != "out" {
		t.Errorf("expected out dir for static export, got %s", fp.OutputDir)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template for static export, got %s", fp.DockerTemplate)
	}
}

func TestDetectReactCRA(t *testing.T) {
	dir := t.TempDir()
	writePkg(dir, map[string]interface{}{
		"name":         "my-cra-app",
		"dependencies": map[string]string{"react": "18.0.0", "react-scripts": "5.0.0"},
		"scripts":      map[string]string{"build": "react-scripts build"},
	})

	fp := Detect(dir)
	if fp.Framework != ReactCRA {
		t.Errorf("expected ReactCRA, got %s", fp.Framework)
	}
	if fp.OutputDir != "build" {
		t.Errorf("expected build, got %s", fp.OutputDir)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template, got %s", fp.DockerTemplate)
	}
}

func TestDetectAstroStatic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "astro.config.mjs"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "my-astro-site",
		"devDependencies": map[string]string{"astro": "4.0.0"},
		"scripts":         map[string]string{"build": "astro build"},
	})

	fp := Detect(dir)
	if fp.Framework != Astro {
		t.Errorf("expected Astro, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeStatic {
		t.Errorf("expected static runtime, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template, got %s", fp.DockerTemplate)
	}
}

func TestDetectAstroSSR(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "astro.config.mjs"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "my-astro-ssr",
		"dependencies":    map[string]string{"astro": "4.0.0", "@astrojs/node": "8.0.0"},
		"devDependencies": map[string]string{},
		"scripts":         map[string]string{"build": "astro build"},
	})

	fp := Detect(dir)
	if fp.Framework != Astro {
		t.Errorf("expected Astro, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeSSR {
		t.Errorf("expected SSR runtime for Astro node adapter, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSSR {
		t.Errorf("expected node-ssr template, got %s", fp.DockerTemplate)
	}
}

func TestDetectSvelteKitNode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "svelte.config.js"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "my-sveltekit",
		"devDependencies": map[string]string{"@sveltejs/kit": "2.0.0", "@sveltejs/adapter-node": "1.0.0"},
		"scripts":         map[string]string{"build": "vite build"},
	})

	fp := Detect(dir)
	if fp.Framework != SvelteKit {
		t.Errorf("expected SvelteKit, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeSSR {
		t.Errorf("expected SSR runtime for node adapter, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSSR {
		t.Errorf("expected node-ssr template, got %s", fp.DockerTemplate)
	}
}

func TestDetectSvelteKitStatic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "svelte.config.js"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "my-sveltekit-static",
		"devDependencies": map[string]string{"@sveltejs/kit": "2.0.0", "@sveltejs/adapter-static": "3.0.0"},
		"scripts":         map[string]string{"build": "vite build"},
	})

	fp := Detect(dir)
	if fp.Framework != SvelteKit {
		t.Errorf("expected SvelteKit, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeStatic {
		t.Errorf("expected static runtime for static adapter, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template, got %s", fp.DockerTemplate)
	}
}

func TestDetectNuxt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nuxt.config.ts"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":         "my-nuxt-app",
		"dependencies": map[string]string{"nuxt": "3.0.0"},
		"scripts":      map[string]string{"build": "nuxt build"},
	})

	fp := Detect(dir)
	if fp.Framework != Nuxt {
		t.Errorf("expected Nuxt, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeSSR {
		t.Errorf("expected SSR runtime, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSSR {
		t.Errorf("expected node-ssr template, got %s", fp.DockerTemplate)
	}
	if fp.OutputDir != ".output" {
		t.Errorf("expected .output, got %s", fp.OutputDir)
	}
}

func TestDetectRemix(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "remix.config.js"), []byte("module.exports = {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":         "my-remix-app",
		"dependencies": map[string]string{"@remix-run/react": "2.0.0", "@remix-run/node": "2.0.0"},
		"scripts":      map[string]string{"build": "remix build", "start": "remix-serve build"},
	})

	fp := Detect(dir)
	if fp.Framework != Remix {
		t.Errorf("expected Remix, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeSSR {
		t.Errorf("expected SSR runtime, got %s", fp.RuntimeType)
	}
	if fp.DockerTemplate != TemplateSSR {
		t.Errorf("expected node-ssr template, got %s", fp.DockerTemplate)
	}
}

func TestDetectStaticHTML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644)

	fp := Detect(dir)
	if fp.Framework != Static {
		t.Errorf("expected Static, got %s", fp.Framework)
	}
	if fp.RuntimeType != RuntimeStatic {
		t.Errorf("expected static runtime, got %s", fp.RuntimeType)
	}
}

func TestDetectEmptyDir(t *testing.T) {
	dir := t.TempDir()
	fp := Detect(dir)
	if fp.Framework != Static {
		t.Errorf("expected Static fallback, got %s", fp.Framework)
	}
	if fp.DockerTemplate != TemplateSPA {
		t.Errorf("expected spa template fallback, got %s", fp.DockerTemplate)
	}
}

// --- Package manager tests ---

func TestDetectPackageManagerPnpm(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	if got := detectPackageManager(dir); got != "pnpm" {
		t.Errorf("expected pnpm, got %s", got)
	}
}

func TestDetectPackageManagerYarn(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	if got := detectPackageManager(dir); got != "yarn" {
		t.Errorf("expected yarn, got %s", got)
	}
}

func TestDetectPackageManagerBun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	if got := detectPackageManager(dir); got != "bun" {
		t.Errorf("expected bun, got %s", got)
	}
}

func TestDetectPackageManagerNpmDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	if got := detectPackageManager(dir); got != "npm" {
		t.Errorf("expected npm, got %s", got)
	}
}

// --- Monorepo tests ---

func TestDetectMonorepoWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writePkg(dir, map[string]interface{}{
		"name":       "my-monorepo",
		"workspaces": []string{"apps/*"},
	})
	os.MkdirAll(filepath.Join(dir, "apps", "web"), 0755)
	os.WriteFile(filepath.Join(dir, "apps", "web", "package.json"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(dir, "apps", "admin"), 0755)
	os.WriteFile(filepath.Join(dir, "apps", "admin", "package.json"), []byte("{}"), 0644)

	fp := Detect(dir)
	if !fp.IsMonorepo {
		t.Error("expected monorepo=true")
	}
	if len(fp.AppPaths) != 2 {
		t.Errorf("expected 2 apps, got %d: %v", len(fp.AppPaths), fp.AppPaths)
	}
}

func TestDetectMonorepoTurbo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "turbo.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	fp := Detect(dir)
	if !fp.IsMonorepo {
		t.Error("expected monorepo=true for turbo.json")
	}
}

// --- Confidence tests ---

func TestHighConfidenceMultipleSignals(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte("export default {}"), 0644)
	writePkg(dir, map[string]interface{}{
		"name":            "high-confidence",
		"devDependencies": map[string]string{"vite": "5.0.0"},
		"scripts":         map[string]string{"build": "vite build"},
	})

	fp := Detect(dir)
	if fp.Confidence < 70 {
		t.Errorf("expected high confidence (>=70), got %d", fp.Confidence)
	}
	if fp.ConfidenceLabel() != "high" {
		t.Errorf("expected 'high' label, got '%s'", fp.ConfidenceLabel())
	}
}

func TestLowConfidenceNoSignals(t *testing.T) {
	dir := t.TempDir()
	fp := Detect(dir)
	if fp.Confidence > 30 {
		t.Errorf("expected low confidence for empty dir, got %d", fp.Confidence)
	}
}

// --- Dockerfile generation tests ---

func TestGenerateDockerfileSPA(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:      Vite,
		PackageManager: "pnpm",
		NodeVersion:    "20",
		RuntimeType:    RuntimeStatic,
		BuildCommand:   "pnpm run build",
		OutputDir:      "dist",
		DockerTemplate: TemplateSPA,
	}

	content, err := fp.GenerateDockerfile()
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	// Check PM-aware install
	if !strings.Contains(content, "pnpm install --frozen-lockfile") {
		t.Error("expected pnpm install command in Dockerfile")
	}
	if !strings.Contains(content, "pnpm-lock.yaml") {
		t.Error("expected pnpm-lock.yaml in COPY instruction")
	}
	// Check nginx stage
	if !strings.Contains(content, "nginx:alpine") {
		t.Error("expected nginx stage in SPA template")
	}
	if !strings.Contains(content, "/app/dist") {
		t.Error("expected dist output dir")
	}
}

func TestGenerateDockerfileNextJS(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:      NextJS,
		PackageManager: "npm",
		NodeVersion:    "20.11.0",
		RuntimeType:    RuntimeSSR,
		BuildCommand:   "npm run build",
		OutputDir:      ".next",
		DockerTemplate: TemplateNextJS,
	}

	content, err := fp.GenerateDockerfile()
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	if !strings.Contains(content, "node:20-alpine") {
		t.Error("expected major node version 20")
	}
	if !strings.Contains(content, "NEXT_TELEMETRY_DISABLED") {
		t.Error("expected Next.js telemetry disabled")
	}
	if !strings.Contains(content, "standalone") {
		t.Error("expected standalone copy in Next.js template")
	}
	if !strings.Contains(content, "EXPOSE 3000") {
		t.Error("expected port 3000 for Next.js")
	}
}

func TestGenerateDockerfileNodeSSR(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:      Nuxt,
		PackageManager: "yarn",
		NodeVersion:    "18",
		RuntimeType:    RuntimeSSR,
		BuildCommand:   "yarn run build",
		OutputDir:      ".output",
		StartCommand:   "node .output/server/index.mjs",
		DockerTemplate: TemplateSSR,
	}

	content, err := fp.GenerateDockerfile()
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}

	if !strings.Contains(content, "node:18-alpine") {
		t.Error("expected node 18")
	}
	if !strings.Contains(content, "yarn install --frozen-lockfile") {
		t.Error("expected yarn install command")
	}
	if !strings.Contains(content, "yarn.lock") {
		t.Error("expected yarn.lock in COPY instruction")
	}
	if !strings.Contains(content, "node:18-slim") {
		t.Error("expected slim runner stage for SSR")
	}
	if !strings.Contains(content, "node .output/server/index.mjs") {
		t.Error("expected start command in CMD")
	}
}

// --- NeedsUserInput tests ---

func TestNeedsUserInputDockerfile(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:     Vite,
		Confidence:    90,
		HasDockerfile: true,
	}
	reasons := fp.NeedsUserInput()
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "Dockerfile") {
			found = true
		}
	}
	if !found {
		t.Error("expected Dockerfile reason in NeedsUserInput")
	}
}

func TestNeedsUserInputLowConfidence(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:  Unknown,
		Confidence: 20,
	}
	reasons := fp.NeedsUserInput()
	if len(reasons) < 2 {
		t.Errorf("expected at least 2 reasons (low confidence + unknown framework), got %d", len(reasons))
	}
}

func TestNeedsUserInputMonorepo(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:  Vite,
		Confidence: 90,
		IsMonorepo: true,
		AppPaths:   []string{"apps/web", "apps/admin"},
	}
	reasons := fp.NeedsUserInput()
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "monorepo") {
			found = true
		}
	}
	if !found {
		t.Error("expected monorepo reason")
	}
}

// --- Node version extraction ---

func TestExtractMajorNodeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"20.11.0", "20"},
		{">=18.0.0", "18"},
		{"^20.0.0", "20"},
		{"v18.17.0", "18"},
		{"20", "20"},
		{"", "20"},
	}
	for _, tt := range tests {
		got := extractMajorNodeVersion(tt.input)
		if got != tt.want {
			t.Errorf("extractMajorNodeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- PM helpers tests ---

func TestInstallCommand(t *testing.T) {
	tests := []struct {
		pm   string
		want string
	}{
		{"npm", "npm ci"},
		{"pnpm", "corepack enable && pnpm install --frozen-lockfile"},
		{"yarn", "yarn install --frozen-lockfile"},
		{"bun", "npm i -g bun && bun install --frozen-lockfile"},
	}
	for _, tt := range tests {
		got := InstallCommand(tt.pm)
		if got != tt.want {
			t.Errorf("InstallCommand(%q) = %q, want %q", tt.pm, got, tt.want)
		}
	}
}

func TestLockFileCopy(t *testing.T) {
	tests := []struct {
		pm   string
		want string
	}{
		{"npm", "COPY package*.json ./"},
		{"pnpm", "COPY package.json pnpm-lock.yaml ./"},
		{"yarn", "COPY package.json yarn.lock ./"},
		{"bun", "COPY package.json bun.lockb ./"},
	}
	for _, tt := range tests {
		got := LockFileCopy(tt.pm)
		if got != tt.want {
			t.Errorf("LockFileCopy(%q) = %q, want %q", tt.pm, got, tt.want)
		}
	}
}

// --- Evidence + Summary ---

func TestSummaryContainsDockerStrategy(t *testing.T) {
	fp := &ProjectFingerprint{
		Framework:      Vite,
		PackageManager: "pnpm",
		RuntimeType:    RuntimeStatic,
		BuildCommand:   "pnpm run build",
		OutputDir:      "dist",
		DockerTemplate: TemplateSPA,
		Confidence:     85,
	}
	summary := fp.Summary()
	if !strings.Contains(summary, "spa") {
		t.Error("summary should contain Docker strategy")
	}
	if !strings.Contains(summary, "85%") {
		t.Error("summary should contain confidence percentage")
	}
}

func TestDockerStrategyLabel(t *testing.T) {
	tests := []struct {
		tmpl DockerTemplate
		want string
	}{
		{TemplateSPA, "multi-stage static nginx"},
		{TemplateNextJS, "Next.js standalone server"},
		{TemplateSSR, "Node.js SSR server"},
	}
	for _, tt := range tests {
		fp := &ProjectFingerprint{DockerTemplate: tt.tmpl}
		got := fp.DockerStrategyLabel()
		if got != tt.want {
			t.Errorf("DockerStrategyLabel(%s) = %q, want %q", tt.tmpl, got, tt.want)
		}
	}
}

// --- Existing Dockerfile detection ---

func TestDetectExistingDockerfile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:20"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	fp := Detect(dir)
	if !fp.HasDockerfile {
		t.Error("expected HasDockerfile=true")
	}
}

// --- helpers ---

func writePkg(dir string, data map[string]interface{}) {
	bytes, _ := json.Marshal(data)
	os.WriteFile(filepath.Join(dir, "package.json"), bytes, 0644)
}

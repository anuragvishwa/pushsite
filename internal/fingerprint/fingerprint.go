package fingerprint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// Framework represents a detected framework
type Framework string

const (
	Vite      Framework = "vite"
	NextJS    Framework = "nextjs"
	ReactCRA  Framework = "react-cra"
	Astro     Framework = "astro"
	SvelteKit Framework = "sveltekit"
	Nuxt      Framework = "nuxt"
	Remix     Framework = "remix"
	Static    Framework = "static"
	Unknown   Framework = "unknown"
)

// RuntimeType represents how the app runs in production
type RuntimeType string

const (
	RuntimeStatic RuntimeType = "static"  // nginx serves files
	RuntimeSSR    RuntimeType = "ssr"     // Node server
	RuntimeHybrid RuntimeType = "hybrid"  // static export from SSR framework
)

// DockerTemplate represents which Dockerfile pattern to use
type DockerTemplate string

const (
	TemplateSPA    DockerTemplate = "spa"      // multi-stage: build → nginx
	TemplateNextJS DockerTemplate = "nextjs"   // standalone next.js server
	TemplateSSR    DockerTemplate = "node-ssr" // generic Node SSR
)

// ProjectFingerprint is the full detection result
type ProjectFingerprint struct {
	Framework      Framework      `json:"framework"`
	PackageManager string         `json:"package_manager"`
	NodeVersion    string         `json:"node_version"`
	RuntimeType    RuntimeType    `json:"runtime_type"`
	BuildCommand   string         `json:"build_command"`
	StartCommand   string         `json:"start_command"`
	OutputDir      string         `json:"output_dir"`
	DockerTemplate DockerTemplate `json:"docker_template"`
	Confidence     int            `json:"confidence"` // 0-100
	Evidence       []string       `json:"evidence"`
	IsMonorepo     bool           `json:"is_monorepo"`
	AppPaths       []string       `json:"app_paths,omitempty"` // for monorepos
	HasDockerfile  bool           `json:"has_dockerfile"`
}

// signal represents one detection signal with a weight
type signal struct {
	framework Framework
	runtime   RuntimeType
	weight    int // strong=30, medium=15, weak=5
	reason    string
}

// Detect performs a full project fingerprint of the given directory
func Detect(dir string) *ProjectFingerprint {
	fp := &ProjectFingerprint{
		Framework:  Unknown,
		RuntimeType: RuntimeStatic,
		Confidence: 0,
	}

	var signals []signal

	// --- 1. Framework config files (strong signals, weight=30) ---
	configSignals := []struct {
		files     []string
		framework Framework
		runtime   RuntimeType
	}{
		{[]string{"next.config.js", "next.config.mjs", "next.config.ts"}, NextJS, RuntimeSSR},
		{[]string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}, Vite, RuntimeStatic},
		{[]string{"astro.config.mjs", "astro.config.js", "astro.config.ts"}, Astro, RuntimeStatic},
		{[]string{"svelte.config.js"}, SvelteKit, RuntimeSSR},
		{[]string{"nuxt.config.ts", "nuxt.config.js"}, Nuxt, RuntimeSSR},
		{[]string{"remix.config.js"}, Remix, RuntimeSSR},
	}

	for _, cs := range configSignals {
		for _, f := range cs.files {
			if fileExists(dir, f) {
				signals = append(signals, signal{cs.framework, cs.runtime, 30, fmt.Sprintf("found config file: %s", f)})
				break // only count once per framework
			}
		}
	}

	// --- 2. package.json analysis (medium signals, weight=15) ---
	pkg := readPackageJSON(dir)
	if pkg != nil {
		// Name / version
		if pkg.Name != "" {
			fp.Evidence = append(fp.Evidence, "package.json name: "+pkg.Name)
		}

		// Dependency checks
		depChecks := []struct {
			dep       string
			where     string // deps, devDeps
			framework Framework
			runtime   RuntimeType
			weight    int
		}{
			{"next", "deps", NextJS, RuntimeSSR, 15},
			{"vite", "devDeps", Vite, RuntimeStatic, 15},
			{"react-scripts", "deps", ReactCRA, RuntimeStatic, 15},
			{"react-scripts", "devDeps", ReactCRA, RuntimeStatic, 15},
			{"astro", "deps", Astro, RuntimeStatic, 15},
			{"astro", "devDeps", Astro, RuntimeStatic, 15},
			{"@sveltejs/kit", "deps", SvelteKit, RuntimeSSR, 15},
			{"@sveltejs/kit", "devDeps", SvelteKit, RuntimeSSR, 15},
			{"nuxt", "deps", Nuxt, RuntimeSSR, 15},
			{"nuxt", "devDeps", Nuxt, RuntimeSSR, 15},
			{"@remix-run/react", "deps", Remix, RuntimeSSR, 15},
			{"@remix-run/node", "deps", Remix, RuntimeSSR, 15},
		}

		for _, dc := range depChecks {
			src := pkg.Dependencies
			if dc.where == "devDeps" {
				src = pkg.DevDependencies
			}
			if _, ok := src[dc.dep]; ok {
				signals = append(signals, signal{dc.framework, dc.runtime, dc.weight,
					fmt.Sprintf("found %s in %s: %s", dc.dep, dc.where, dc.dep)})
			}
		}

		// Build script analysis
		if buildScript, ok := pkg.Scripts["build"]; ok {
			fp.Evidence = append(fp.Evidence, "build script: "+buildScript)

			if strings.Contains(buildScript, "next build") || strings.Contains(buildScript, "next") {
				signals = append(signals, signal{NextJS, RuntimeSSR, 10, "build script contains 'next'"})
			}
			if strings.Contains(buildScript, "vite build") || strings.Contains(buildScript, "vite") {
				signals = append(signals, signal{Vite, RuntimeStatic, 10, "build script contains 'vite'"})
			}
			if strings.Contains(buildScript, "react-scripts build") {
				signals = append(signals, signal{ReactCRA, RuntimeStatic, 10, "build script contains 'react-scripts'"})
			}
			if strings.Contains(buildScript, "astro build") {
				signals = append(signals, signal{Astro, RuntimeStatic, 10, "build script contains 'astro'"})
			}
			if strings.Contains(buildScript, "nuxt build") {
				signals = append(signals, signal{Nuxt, RuntimeSSR, 10, "build script contains 'nuxt'"})
			}
			if strings.Contains(buildScript, "remix build") {
				signals = append(signals, signal{Remix, RuntimeSSR, 10, "build script contains 'remix'"})
			}
		}

		// Start script analysis — determines runtime
		if startScript, ok := pkg.Scripts["start"]; ok {
			fp.StartCommand = startScript
			if strings.Contains(startScript, "next start") {
				signals = append(signals, signal{NextJS, RuntimeSSR, 10, "start script: next start"})
			}
			if strings.Contains(startScript, "node server") || strings.Contains(startScript, "node .") {
				fp.Evidence = append(fp.Evidence, "custom node server detected")
			}
		}

		// Node version from engines
		if pkg.Engines.Node != "" {
			fp.NodeVersion = pkg.Engines.Node
			fp.Evidence = append(fp.Evidence, "engines.node: "+pkg.Engines.Node)
		}

		// TypeScript
		if _, ok := pkg.DevDependencies["typescript"]; ok {
			fp.Evidence = append(fp.Evidence, "TypeScript: yes")
		}

		// --- Next.js runtime detection: SSR vs static export ---
		if hasSignalFor(signals, NextJS) {
			fp.RuntimeType = RuntimeSSR // default for Next.js is SSR

			// Check for static export config
			for _, f := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
				content, err := os.ReadFile(filepath.Join(dir, f))
				if err == nil {
					s := string(content)
					if strings.Contains(s, `output: "export"`) || strings.Contains(s, `output: 'export'`) {
						signals = append(signals, signal{NextJS, RuntimeHybrid, 20, "next.config has output: 'export' (static export)"})
					}
					if strings.Contains(s, "standalone") {
						fp.Evidence = append(fp.Evidence, "next.config uses standalone output")
					}
				}
			}
		}

		// --- Astro output mode ---
		if hasSignalFor(signals, Astro) {
			// Check for SSR adapter
			if _, ok := pkg.Dependencies["@astrojs/node"]; ok {
				signals = append(signals, signal{Astro, RuntimeSSR, 20, "Astro node adapter detected"})
			} else if _, ok := pkg.DevDependencies["@astrojs/node"]; ok {
				signals = append(signals, signal{Astro, RuntimeSSR, 20, "Astro node adapter detected"})
			}
		}

		// --- SvelteKit adapter ---
		if hasSignalFor(signals, SvelteKit) {
			if _, ok := pkg.DevDependencies["@sveltejs/adapter-static"]; ok {
				signals = append(signals, signal{SvelteKit, RuntimeStatic, 20, "SvelteKit uses static adapter"})
			}
			if _, ok := pkg.DevDependencies["@sveltejs/adapter-node"]; ok {
				signals = append(signals, signal{SvelteKit, RuntimeSSR, 20, "SvelteKit uses node adapter"})
			}
		}
	}

	// --- 3. Folder conventions (weak signals, weight=5) ---
	if fileExists(dir, "pages") || fileExists(dir, "app") {
		if hasSignalFor(signals, NextJS) {
			signals = append(signals, signal{NextJS, RuntimeSSR, 5, "found pages/ or app/ directory"})
		}
	}
	if fileExists(dir, "src/pages") {
		if hasSignalFor(signals, Astro) {
			signals = append(signals, signal{Astro, RuntimeStatic, 5, "found src/pages/ (Astro convention)"})
		}
	}
	if fileExists(dir, "src/routes") {
		if hasSignalFor(signals, SvelteKit) {
			signals = append(signals, signal{SvelteKit, RuntimeSSR, 5, "found src/routes/ (SvelteKit convention)"})
		}
	}
	if fileExists(dir, "index.html") {
		signals = append(signals, signal{Static, RuntimeStatic, 5, "found index.html in root"})
	}

	// --- 4. Monorepo detection ---
	fp.IsMonorepo = detectMonorepo(dir, pkg)
	if fp.IsMonorepo {
		fp.AppPaths = findApps(dir, pkg)
		fp.Evidence = append(fp.Evidence, fmt.Sprintf("monorepo detected: %d app(s)", len(fp.AppPaths)))
	}

	// --- 5. Existing Dockerfile detection ---
	fp.HasDockerfile = fileExists(dir, "Dockerfile")
	if fp.HasDockerfile {
		fp.Evidence = append(fp.Evidence, "existing Dockerfile found")
	}

	// --- Score and rank ---
	fp.Framework, fp.RuntimeType, fp.Confidence = scoreSignals(signals)

	// Add signal evidence
	for _, s := range signals {
		fp.Evidence = append(fp.Evidence, fmt.Sprintf("[%s +%d] %s", s.framework, s.weight, s.reason))
	}

	// --- Derive build/output/docker from framework+runtime ---
	fp.PackageManager = detectPackageManager(dir)
	pm := fp.PackageManager
	if pm == "" {
		pm = "npm"
	}

	fillBuildConfig(fp, pm)
	fillDockerTemplate(fp)

	// Node version from .nvmrc / .node-version
	if fp.NodeVersion == "" {
		fp.NodeVersion = detectNodeVersion(dir)
	}

	return fp
}

// scoreSignals tallies signals and returns winner
func scoreSignals(signals []signal) (Framework, RuntimeType, int) {
	if len(signals) == 0 {
		return Static, RuntimeStatic, 20
	}

	type score struct {
		total   int
		runtime RuntimeType
		count   int
	}

	scores := make(map[Framework]*score)
	for _, s := range signals {
		if _, ok := scores[s.framework]; !ok {
			scores[s.framework] = &score{}
		}
		scores[s.framework].total += s.weight
		scores[s.framework].count++
		// Last runtime signal wins (higher weight signals come from config files)
		if s.weight >= scores[s.framework].total/scores[s.framework].count {
			scores[s.framework].runtime = s.runtime
		}
	}

	// Find winner
	var winner Framework
	var bestScore int
	var winnerRuntime RuntimeType
	for fw, sc := range scores {
		if sc.total > bestScore {
			bestScore = sc.total
			winner = fw
			winnerRuntime = sc.runtime
		}
	}

	// Runtime: prefer the highest-weighted signal for the winner
	for _, s := range signals {
		if s.framework == winner && s.weight >= 15 {
			winnerRuntime = s.runtime
		}
	}

	// Confidence: 0-100 based on total score
	confidence := bestScore * 2
	if confidence > 100 {
		confidence = 100
	}
	// Boost if multiple signals agree
	if scores[winner].count >= 3 {
		confidence += 10
	}
	if confidence > 100 {
		confidence = 100
	}

	return winner, winnerRuntime, confidence
}

// fillBuildConfig sets BuildCommand, OutputDir, StartCommand
func fillBuildConfig(fp *ProjectFingerprint, pm string) {
	run := pm + " run build"
	if pm == "npm" {
		run = "npm run build"
	}

	switch fp.Framework {
	case Vite:
		fp.BuildCommand = run
		fp.OutputDir = "dist"
	case NextJS:
		fp.BuildCommand = run
		if fp.RuntimeType == RuntimeHybrid {
			fp.OutputDir = "out"
		} else {
			fp.OutputDir = ".next"
		}
		fp.StartCommand = "node server.js"
	case ReactCRA:
		fp.BuildCommand = run
		fp.OutputDir = "build"
	case Astro:
		fp.BuildCommand = run
		fp.OutputDir = "dist"
		if fp.RuntimeType == RuntimeSSR {
			fp.StartCommand = "node ./dist/server/entry.mjs"
		}
	case SvelteKit:
		fp.BuildCommand = run
		if fp.RuntimeType == RuntimeStatic {
			fp.OutputDir = "build"
		} else {
			fp.OutputDir = "build"
			fp.StartCommand = "node build"
		}
	case Nuxt:
		fp.BuildCommand = run
		fp.OutputDir = ".output"
		fp.StartCommand = "node .output/server/index.mjs"
	case Remix:
		fp.BuildCommand = run
		fp.OutputDir = "build"
		fp.StartCommand = pm + " run start"
	case Static:
		fp.BuildCommand = ""
		fp.OutputDir = "."
	default:
		fp.BuildCommand = run
		fp.OutputDir = "dist"
	}
}

// fillDockerTemplate selects the right Docker pattern
func fillDockerTemplate(fp *ProjectFingerprint) {
	switch fp.RuntimeType {
	case RuntimeStatic, RuntimeHybrid:
		fp.DockerTemplate = TemplateSPA
	case RuntimeSSR:
		if fp.Framework == NextJS {
			fp.DockerTemplate = TemplateNextJS
		} else {
			fp.DockerTemplate = TemplateSSR
		}
	default:
		fp.DockerTemplate = TemplateSPA
	}
}

// Summary returns a human-friendly detection summary
func (fp *ProjectFingerprint) Summary() string {
	lines := []string{
		fmt.Sprintf("  Framework:      %s", fp.Framework),
		fmt.Sprintf("  Runtime:        %s", fp.RuntimeType),
		fmt.Sprintf("  Package manager: %s", fp.PackageManager),
	}
	if fp.BuildCommand != "" {
		lines = append(lines, fmt.Sprintf("  Build:          %s → %s", fp.BuildCommand, fp.OutputDir))
	}
	if fp.StartCommand != "" {
		lines = append(lines, fmt.Sprintf("  Start:          %s", fp.StartCommand))
	}
	lines = append(lines, fmt.Sprintf("  Docker strategy: %s", fp.DockerTemplate))
	lines = append(lines, fmt.Sprintf("  Confidence:     %d%%", fp.Confidence))

	if fp.NodeVersion != "" {
		lines = append(lines, fmt.Sprintf("  Node:           %s", fp.NodeVersion))
	}
	if fp.IsMonorepo {
		lines = append(lines, fmt.Sprintf("  Monorepo:       yes (%d apps)", len(fp.AppPaths)))
	}

	return strings.Join(lines, "\n")
}

// ConfidenceLabel returns low/medium/high
func (fp *ProjectFingerprint) ConfidenceLabel() string {
	if fp.Confidence >= 70 {
		return "high"
	}
	if fp.Confidence >= 40 {
		return "medium"
	}
	return "low"
}

// NeedsConfirmation returns true if user should confirm before proceeding
func (fp *ProjectFingerprint) NeedsConfirmation() bool {
	return fp.Confidence < 60 || fp.IsMonorepo || fp.HasDockerfile
}

// NeedsUserInput returns a list of reasons the user should be prompted
func (fp *ProjectFingerprint) NeedsUserInput() []string {
	var reasons []string
	if fp.Confidence < 40 {
		reasons = append(reasons, "framework confidence is low")
	}
	if fp.IsMonorepo && len(fp.AppPaths) > 1 {
		reasons = append(reasons, fmt.Sprintf("monorepo has %d app folders", len(fp.AppPaths)))
	}
	if fp.HasDockerfile {
		reasons = append(reasons, "project already has a Dockerfile")
	}
	if fp.Framework == Unknown {
		reasons = append(reasons, "could not identify framework")
	}
	return reasons
}

// --- Dockerfile generation ---

// dockerTemplateData holds all variables for Dockerfile rendering
type dockerTemplateData struct {
	NodeVersion    string
	InstallCmd     string
	LockFileCopy   string
	BuildCmd       string
	OutputDir      string
	StartCmd       string
	Port           int
}

// InstallCommand returns the PM-aware install command for Docker
func InstallCommand(pm string) string {
	switch pm {
	case "pnpm":
		return "corepack enable && pnpm install --frozen-lockfile"
	case "yarn":
		return "yarn install --frozen-lockfile"
	case "bun":
		return "npm i -g bun && bun install --frozen-lockfile"
	default:
		return "npm ci"
	}
}

// LockFileCopy returns the COPY instruction for the lock file based on PM
func LockFileCopy(pm string) string {
	switch pm {
	case "pnpm":
		return "COPY package.json pnpm-lock.yaml ./"
	case "yarn":
		return "COPY package.json yarn.lock ./"
	case "bun":
		return "COPY package.json bun.lockb ./"
	default:
		return "COPY package*.json ./"
	}
}

// GenerateDockerfile renders a Dockerfile using the fingerprint data
func (fp *ProjectFingerprint) GenerateDockerfile() (string, error) {
	nodeVer := fp.NodeVersion
	if nodeVer == "" {
		nodeVer = "20"
	}
	// Extract major version only (e.g. "20.11.0" → "20", ">=20.0.0" → "20")
	nodeVer = extractMajorNodeVersion(nodeVer)

	pm := fp.PackageManager
	if pm == "" {
		pm = "npm"
	}

	data := dockerTemplateData{
		NodeVersion:  nodeVer,
		InstallCmd:   InstallCommand(pm),
		LockFileCopy: LockFileCopy(pm),
		BuildCmd:     fp.BuildCommand,
		OutputDir:    fp.OutputDir,
		StartCmd:     fp.StartCommand,
	}

	var tmplStr string
	switch fp.DockerTemplate {
	case TemplateNextJS:
		data.Port = 3000
		tmplStr = nextjsDockerTemplate
	case TemplateSSR:
		data.Port = 3000
		if data.StartCmd == "" {
			data.StartCmd = "node server.js"
		}
		tmplStr = nodeSSRDockerTemplate
	default: // TemplateSPA
		data.Port = 80
		tmplStr = spaDockerTemplate
	}

	tmpl, err := template.New("dockerfile").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render Dockerfile: %w", err)
	}

	return buf.String(), nil
}

// DockerStrategyLabel returns a human-readable label for the Docker strategy
func (fp *ProjectFingerprint) DockerStrategyLabel() string {
	switch fp.DockerTemplate {
	case TemplateSPA:
		return "multi-stage static nginx"
	case TemplateNextJS:
		return "Next.js standalone server"
	case TemplateSSR:
		return "Node.js SSR server"
	default:
		return "multi-stage static nginx"
	}
}

func extractMajorNodeVersion(ver string) string {
	// Strip common prefixes
	ver = strings.TrimPrefix(ver, ">=")
	ver = strings.TrimPrefix(ver, "^")
	ver = strings.TrimPrefix(ver, "~")
	ver = strings.TrimPrefix(ver, "v")
	ver = strings.TrimSpace(ver)
	// Take first segment
	if idx := strings.IndexByte(ver, '.'); idx > 0 {
		return ver[:idx]
	}
	if ver == "" {
		return "20"
	}
	return ver
}

// --- Dockerfile templates ---

var spaDockerTemplate = `FROM node:{{.NodeVersion}}-alpine AS builder
WORKDIR /app
{{.LockFileCopy}}
RUN {{.InstallCmd}}
COPY . .
RUN {{.BuildCmd}}

FROM nginx:alpine
COPY --from=builder /app/{{.OutputDir}} /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

var nextjsDockerTemplate = `FROM node:{{.NodeVersion}}-alpine AS deps
WORKDIR /app
{{.LockFileCopy}}
RUN {{.InstallCmd}}

FROM node:{{.NodeVersion}}-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED 1
RUN {{.BuildCmd}}

FROM node:{{.NodeVersion}}-alpine AS runner
WORKDIR /app
ENV NODE_ENV production
ENV NEXT_TELEMETRY_DISABLED 1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE {{.Port}}
ENV PORT {{.Port}}
CMD ["node", "server.js"]
`

var nodeSSRDockerTemplate = `FROM node:{{.NodeVersion}}-alpine AS deps
WORKDIR /app
{{.LockFileCopy}}
RUN {{.InstallCmd}}

FROM node:{{.NodeVersion}}-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN {{.BuildCmd}}

FROM node:{{.NodeVersion}}-slim AS runner
WORKDIR /app
ENV NODE_ENV=production

COPY --from=builder /app/{{.OutputDir}} ./{{.OutputDir}}
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./

EXPOSE {{.Port}}
CMD {{.StartCmd}}
`

// --- Helpers ---

type packageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      json.RawMessage   `json:"workspaces"`
	Engines         struct {
		Node string `json:"node"`
	} `json:"engines"`
}

func readPackageJSON(dir string) *packageJSON {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	return &pkg
}

func hasSignalFor(signals []signal, fw Framework) bool {
	for _, s := range signals {
		if s.framework == fw {
			return true
		}
	}
	return false
}

func detectPackageManager(dir string) string {
	lockFiles := []struct {
		file    string
		manager string
	}{
		{"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, lf := range lockFiles {
		if fileExists(dir, lf.file) {
			return lf.manager
		}
	}
	if fileExists(dir, "package.json") {
		return "npm"
	}
	return ""
}

func detectNodeVersion(dir string) string {
	for _, f := range []string{".nvmrc", ".node-version"} {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err == nil {
			ver := strings.TrimSpace(string(content))
			ver = strings.TrimPrefix(ver, "v")
			if ver != "" {
				return ver
			}
		}
	}
	return ""
}

func detectMonorepo(dir string, pkg *packageJSON) bool {
	if pkg != nil && pkg.Workspaces != nil {
		return true
	}
	if fileExists(dir, "pnpm-workspace.yaml") {
		return true
	}
	if fileExists(dir, "lerna.json") {
		return true
	}
	// Turborepo
	if fileExists(dir, "turbo.json") {
		return true
	}
	return false
}

func findApps(dir string, pkg *packageJSON) []string {
	var apps []string

	// Check common app directories
	candidates := []string{"apps", "packages", "frontend", "client", "web"}
	for _, c := range candidates {
		subdir := filepath.Join(dir, c)
		entries, err := os.ReadDir(subdir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				appDir := filepath.Join(c, e.Name())
				// Must have package.json to be an "app"
				if fileExists(dir, filepath.Join(appDir, "package.json")) {
					apps = append(apps, appDir)
				}
			}
		}
	}

	sort.Strings(apps)
	return apps
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

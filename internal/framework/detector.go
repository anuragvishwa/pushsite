package framework

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Framework represents a frontend framework type
type Framework string

const (
	Vite   Framework = "vite"
	NextJS Framework = "nextjs"
	React  Framework = "react"
	Static Framework = "static"
)

// Info holds detected framework metadata
type Info struct {
	Name       Framework
	BuildCmd   string
	OutputDir  string
	IsSSR      bool
	HasTypeScript bool
}

// Detect auto-detects the frontend framework in the given directory
func Detect(dir string) *Info {
	// Check for Next.js first (has next in dependencies)
	if hasFile(dir, "next.config.js") || hasFile(dir, "next.config.mjs") || hasFile(dir, "next.config.ts") {
		return &Info{
			Name:      NextJS,
			BuildCmd:  "npm run build",
			OutputDir: ".next",
			IsSSR:     true,
		}
	}

	// Check for Vite
	if hasFile(dir, "vite.config.js") || hasFile(dir, "vite.config.ts") || hasFile(dir, "vite.config.mjs") {
		return &Info{
			Name:      Vite,
			BuildCmd:  "npm run build",
			OutputDir: "dist",
			IsSSR:     false,
		}
	}

	// Check package.json for hints
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg packageJSON
		if json.Unmarshal(data, &pkg) == nil {
			// Check for Next.js in dependencies
			if _, ok := pkg.Dependencies["next"]; ok {
				return &Info{
					Name:      NextJS,
					BuildCmd:  "npm run build",
					OutputDir: ".next",
					IsSSR:     true,
				}
			}
			if _, ok := pkg.DevDependencies["next"]; ok {
				return &Info{
					Name:      NextJS,
					BuildCmd:  "npm run build",
					OutputDir: ".next",
					IsSSR:     true,
				}
			}

			// Check for Vite
			if _, ok := pkg.DevDependencies["vite"]; ok {
				return &Info{
					Name:      Vite,
					BuildCmd:  "npm run build",
					OutputDir: "dist",
					IsSSR:     false,
				}
			}

			// Check for Create React App (react-scripts)
			if _, ok := pkg.Dependencies["react-scripts"]; ok {
				return &Info{
					Name:      React,
					BuildCmd:  "npm run build",
					OutputDir: "build",
					IsSSR:     false,
				}
			}
			if _, ok := pkg.DevDependencies["react-scripts"]; ok {
				return &Info{
					Name:      React,
					BuildCmd:  "npm run build",
					OutputDir: "build",
					IsSSR:     false,
				}
			}

			// Check for TypeScript
			hasTS := false
			if _, ok := pkg.DevDependencies["typescript"]; ok {
				hasTS = true
			}

			// Generic React project with Vite or other bundler
			if _, ok := pkg.Dependencies["react"]; ok {
				info := &Info{
					Name:          React,
					BuildCmd:      "npm run build",
					OutputDir:     "dist",
					IsSSR:         false,
					HasTypeScript: hasTS,
				}
				// Check if build script exists
				if buildScript, ok := pkg.Scripts["build"]; ok {
					if strings.Contains(buildScript, "vite") {
						info.OutputDir = "dist"
					} else if strings.Contains(buildScript, "react-scripts") {
						info.OutputDir = "build"
					}
				}
				return info
			}
		}
	}

	// Check for static site indicators
	if hasFile(dir, "index.html") {
		return &Info{
			Name:      Static,
			BuildCmd:  "",
			OutputDir: ".",
			IsSSR:     false,
		}
	}

	// Default to static
	return &Info{
		Name:      Static,
		BuildCmd:  "",
		OutputDir: "dist",
		IsSSR:     false,
	}
}

// FrameworkFromString converts a string to a Framework type
func FrameworkFromString(s string) Framework {
	switch strings.ToLower(s) {
	case "vite":
		return Vite
	case "nextjs", "next":
		return NextJS
	case "react", "cra":
		return React
	case "static", "html":
		return Static
	default:
		return Static
	}
}

// BuildOutput returns the default build output directory for a framework
func BuildOutput(f Framework) string {
	switch f {
	case NextJS:
		return ".next"
	case React:
		return "build"
	case Vite:
		return "dist"
	default:
		return "dist"
	}
}

type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func hasFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

package docker

import (
	"strings"
	"testing"
)

func TestGenerateDockerfileSPA(t *testing.T) {
	content, err := GenerateDockerfile("vite", "npm run build", "dist")
	if err != nil {
		t.Fatalf("GenerateDockerfile() error = %v", err)
	}

	checks := []string{
		"FROM node:20-alpine AS builder",
		"RUN npm run build",
		"FROM nginx:alpine",
		"/app/dist",
		"EXPOSE 80",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("SPA Dockerfile missing: %s", check)
		}
	}
}

func TestGenerateDockerfileNextJS(t *testing.T) {
	content, err := GenerateDockerfile("nextjs", "npm run build", ".next")
	if err != nil {
		t.Fatalf("GenerateDockerfile() error = %v", err)
	}

	checks := []string{
		"FROM node:20-alpine AS deps",
		"FROM node:20-alpine AS builder",
		"FROM node:20-alpine AS runner",
		"NEXT_TELEMETRY_DISABLED",
		".next/standalone",
		".next/static",
		"EXPOSE 3000",
		"CMD [\"node\", \"server.js\"]",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("NextJS Dockerfile missing: %s", check)
		}
	}
}

func TestGenerateDockerfileDefault(t *testing.T) {
	content, err := GenerateDockerfile("static", "npm run build", "dist")
	if err != nil {
		t.Fatalf("GenerateDockerfile() error = %v", err)
	}

	// Default should be SPA template (nginx)
	if !strings.Contains(content, "nginx:alpine") {
		t.Error("Default Dockerfile should use nginx")
	}
}

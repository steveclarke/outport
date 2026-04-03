package portinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxWalkDepth limits how far up the directory tree we look for project markers.
	maxWalkDepth = 15
	// maxFileSize is the largest project marker file we'll read (1 MB).
	// Protects against accidentally reading a huge file from an unknown CWD.
	maxFileSize = 1 << 20
)

// safeReadFile reads a file only if it's under maxFileSize. Returns nil on any error.
func safeReadFile(path string) []byte {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxFileSize {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// projectMarkers maps filenames to framework detection functions.
var projectMarkers = []struct {
	file   string
	detect func(dir string) string
}{
	{"package.json", detectNodeFramework},
	{"go.mod", func(string) string { return "Go" }},
	{"Gemfile", detectRubyFramework},
	{"Cargo.toml", func(string) string { return "Rust" }},
	{"pyproject.toml", func(string) string { return "Python" }},
	{"requirements.txt", func(string) string { return "Python" }},
	{"manage.py", func(string) string { return "Django" }},
	{"pom.xml", func(string) string { return "Java" }},
	{"build.gradle", func(string) string { return "Java" }},
}

// detectFramework walks up from cwd looking for project root markers.
// Returns (projectName, frameworkName). Both are empty if no markers found.
func detectFramework(cwd string) (string, string) {
	if cwd == "" {
		return "", ""
	}

	dir := cwd
	for i := 0; i < maxWalkDepth; i++ {
		for _, marker := range projectMarkers {
			path := filepath.Join(dir, marker.file)
			if _, err := os.Stat(path); err == nil {
				framework := marker.detect(dir)
				return filepath.Base(dir), framework
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ""
}

// detectNodeFramework reads package.json and checks dependencies for known frameworks.
func detectNodeFramework(dir string) string {
	data := safeReadFile(filepath.Join(dir, "package.json"))
	if data == nil {
		return "Node.js"
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "Node.js"
	}

	frameworks := []struct {
		pkg  string
		name string
	}{
		{"next", "Next.js"},
		{"nuxt", "Nuxt"},
		{"@sveltejs/kit", "SvelteKit"},
		{"@angular/core", "Angular"},
		{"vue", "Vue"},
		{"svelte", "Svelte"},
		{"express", "Express"},
		{"fastify", "Fastify"},
		{"hono", "Hono"},
		{"@nestjs/core", "NestJS"},
		{"vite", "Vite"},
		{"webpack", "Webpack"},
	}

	for _, f := range frameworks {
		if _, ok := pkg.Dependencies[f.pkg]; ok {
			return f.name
		}
		if _, ok := pkg.DevDependencies[f.pkg]; ok {
			return f.name
		}
	}

	return "Node.js"
}

// detectRubyFramework reads Gemfile and checks for known frameworks.
func detectRubyFramework(dir string) string {
	data := safeReadFile(filepath.Join(dir, "Gemfile"))
	if data == nil {
		return "Ruby"
	}
	content := string(data)
	if strings.Contains(content, `"rails"`) || strings.Contains(content, `'rails'`) {
		return "Rails"
	}
	if strings.Contains(content, `"sinatra"`) || strings.Contains(content, `'sinatra'`) {
		return "Sinatra"
	}
	return "Ruby"
}

// devProcessNames is the allowlist of process names considered "dev processes"
// for orphan detection.
var devProcessNames = map[string]bool{
	"node":     true,
	"ruby":     true,
	"python":   true,
	"python3":  true,
	"go":       true,
	"cargo":    true,
	"deno":     true,
	"bun":      true,
	"java":     true,
	"php":      true,
	"elixir":   true,
	"beam.smp": true,
	"dotnet":   true,
}

// isOrphanProcess returns true when a process is likely an orphaned dev process:
// ppid=1 (reparented to init/launchd) AND a known dev runtime name.
// This heuristic has false positives for intentionally daemonized processes
// (e.g., PM2-managed node) and may miss orphans on Linux where systemd --user
// reparents to a non-PID-1 process.
func isOrphanProcess(ppid int, processName string) bool {
	if ppid != 1 {
		return false
	}
	return devProcessNames[processName]
}

// isZombieProcess returns true if the process state indicates a zombie.
func isZombieProcess(state string) bool {
	return strings.Contains(state, "Z")
}

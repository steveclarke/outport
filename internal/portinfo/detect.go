package portinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// maxWalkDepth limits how far up the directory tree we look for project markers.
const maxWalkDepth = 15

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
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
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
	data, err := os.ReadFile(filepath.Join(dir, "Gemfile"))
	if err != nil {
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

// isOrphanProcess returns true when a process is likely an orphaned dev process.
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

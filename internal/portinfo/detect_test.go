package portinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string // relative path → content
		subdir    string            // if set, use this subdir as CWD instead of root
		wantProj  string
		wantFrame string
	}{
		{
			name: "Next.js from package.json",
			files: map[string]string{
				"package.json": `{"dependencies":{"next":"14.0.0"}}`,
			},
			wantFrame: "Next.js",
		},
		{
			name: "Rails from Gemfile",
			files: map[string]string{
				"Gemfile": `gem "rails", "~> 7.0"`,
			},
			wantFrame: "Rails",
		},
		{
			name: "Go from go.mod",
			files: map[string]string{
				"go.mod": `module github.com/example/myapp`,
			},
			wantFrame: "Go",
		},
		{
			name: "Nuxt from package.json",
			files: map[string]string{
				"package.json": `{"devDependencies":{"nuxt":"3.0.0"}}`,
			},
			wantFrame: "Nuxt",
		},
		{
			name: "Vite from package.json",
			files: map[string]string{
				"package.json": `{"devDependencies":{"vite":"5.0.0"}}`,
			},
			wantFrame: "Vite",
		},
		{
			name: "Django from manage.py",
			files: map[string]string{
				"manage.py": `#!/usr/bin/env python`,
			},
			wantFrame: "Django",
		},
		{
			name: "Rust from Cargo.toml",
			files: map[string]string{
				"Cargo.toml": `[package]`,
			},
			wantFrame: "Rust",
		},
		{
			name: "Express from package.json",
			files: map[string]string{
				"package.json": `{"dependencies":{"express":"4.0.0"}}`,
			},
			wantFrame: "Express",
		},
		{
			name: "no markers returns empty",
			files: map[string]string{
				"readme.md": "hello",
			},
			wantFrame: "",
		},
		{
			name: "walks up from subdirectory",
			files: map[string]string{
				"package.json": `{"dependencies":{"express":"4.0.0"}}`,
			},
			subdir:    "src/app",
			wantFrame: "Express",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, content := range tt.files {
				fullPath := filepath.Join(dir, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			cwd := dir
			if tt.subdir != "" {
				cwd = filepath.Join(dir, tt.subdir)
				if err := os.MkdirAll(cwd, 0755); err != nil {
					t.Fatal(err)
				}
			}

			proj, frame := detectFramework(cwd)

			// When a marker is found, project name = base dir name
			if tt.wantFrame != "" {
				if proj != filepath.Base(dir) {
					t.Errorf("project = %q, want %q", proj, filepath.Base(dir))
				}
			} else {
				if proj != "" {
					t.Errorf("project = %q, want empty", proj)
				}
			}

			if frame != tt.wantFrame {
				t.Errorf("framework = %q, want %q", frame, tt.wantFrame)
			}
		})
	}
}

func TestIsOrphan(t *testing.T) {
	tests := []struct {
		name        string
		ppid        int
		processName string
		want        bool
	}{
		{"node with ppid 1", 1, "node", true},
		{"ruby with ppid 1", 1, "ruby", true},
		{"python with ppid 1", 1, "python3", true},
		{"go with ppid 1", 1, "go", true},
		{"postgres with ppid 1 not dev process", 1, "postgres", false},
		{"node with normal ppid", 1042, "node", false},
		{"system daemon ppid 0", 0, "launchd", false},
		{"chrome with ppid 1 not dev process", 1, "Google Chrome", false},
		{"deno with ppid 1", 1, "deno", true},
		{"bun with ppid 1", 1, "bun", true},
		{"cargo with ppid 1", 1, "cargo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOrphanProcess(tt.ppid, tt.processName)
			if got != tt.want {
				t.Errorf("isOrphanProcess(%d, %q) = %v, want %v", tt.ppid, tt.processName, got, tt.want)
			}
		})
	}
}

func TestIsZombie(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"Z", true},
		{"Zs", true},
		{"S", false},
		{"Ss", false},
		{"R", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := isZombieProcess(tt.state)
			if got != tt.want {
				t.Errorf("isZombieProcess(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

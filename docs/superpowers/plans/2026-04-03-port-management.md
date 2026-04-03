# Port Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `outport ports` command with live process inspection, full machine scan, and process kill capabilities.

**Architecture:** New `internal/portinfo` package handles system-level port scanning via `lsof`/`ps`, with a `Scanner` interface for testability. CLI commands in `cmd/ports.go` and `cmd/ports_kill.go` consume `portinfo` and render output using existing patterns. Doctor integration adds orphan detection to project checks.

**Tech Stack:** Go, lsof, ps, Cobra CLI, lipgloss terminal styling.

**Spec:** `docs/superpowers/specs/2026-04-03-port-management-design.md`

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `internal/portinfo/portinfo.go` | Types (`ProcessInfo`, `Scanner` interface), `Scan()`, `ScanPorts()`, `Kill()` |
| `internal/portinfo/parse.go` | Parse lsof and ps output into structured data |
| `internal/portinfo/detect.go` | Framework detection (project markers, package.json), orphan/zombie detection |
| `internal/portinfo/system.go` | `SystemScanner` — real lsof/ps exec calls |
| `internal/portinfo/portinfo_test.go` | Tests for scanning, parsing, enrichment |
| `internal/portinfo/parse_test.go` | Tests for lsof/ps output parsing |
| `internal/portinfo/detect_test.go` | Tests for framework detection, orphan detection |
| `cmd/ports.go` | `outport ports` command — listing with `--all`, `--json` |
| `cmd/ports_kill.go` | `outport ports kill` subcommand — service name, port number, `--orphans` |
| `cmd/ports_test.go` | Tests for ports command output and flag handling |
| `cmd/ports_kill_test.go` | Tests for kill command safety checks and flag handling |

### Modified files

| File | Change |
|---|---|
| `cmd/root.go` | No change needed — `ports.go` init() registers via `rootCmd.AddCommand()` |
| `internal/doctor/project.go` | Add orphan check to `ProjectChecks()` |
| `internal/doctor/project_test.go` | Test orphan check |

---

## Task 1: Parse lsof Listening Ports Output

**Files:**
- Create: `internal/portinfo/parse.go`
- Create: `internal/portinfo/parse_test.go`

- [ ] **Step 1: Write the test file with lsof parsing tests**

```go
// internal/portinfo/parse_test.go
package portinfo

import (
	"testing"
)

func TestParseLsofListening(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []lsofEntry
		wantErr bool
	}{
		{
			name: "typical macOS output",
			input: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
postgres    412  steve    5u  IPv4 0xaabbccddee      0t0  TCP 127.0.0.1:5432 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 48291, ProcessName: "node", Port: 13542},
				{PID: 51002, ProcessName: "ruby", Port: 3000},
				{PID: 412, ProcessName: "postgres", Port: 5432},
			},
		},
		{
			name: "IPv6 with bracket notation",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    12345 steve   22u  IPv6 0x1234      0t0  TCP [::1]:8080 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 12345, ProcessName: "node", Port: 8080},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "header only",
			input: "COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME\n",
			want:  nil,
		},
		{
			name: "wildcard listen address",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    99999 steve   22u  IPv4 0x1234      0t0  TCP *:4000 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 99999, ProcessName: "node", Port: 4000},
			},
		},
		{
			name: "duplicate port different PIDs keeps both",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    100 steve   22u  IPv4 0x1234      0t0  TCP *:3000 (LISTEN)
node    200 steve   23u  IPv4 0x5678      0t0  TCP *:3000 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 100, ProcessName: "node", Port: 3000},
				{PID: 200, ProcessName: "node", Port: 3000},
			},
		},
		{
			name: "malformed line skipped",
			input: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
this is not valid lsof output
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 48291, ProcessName: "node", Port: 13542},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsofListening(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLsofListening() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/portinfo/ -run TestParseLsofListening -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Create parse.go with lsofEntry type and parseLsofListening**

```go
// internal/portinfo/parse.go
package portinfo

import (
	"regexp"
	"strconv"
	"strings"
)

// lsofEntry is a raw port→PID mapping extracted from lsof output.
type lsofEntry struct {
	PID         int
	ProcessName string
	Port        int
}

// portPattern matches the port number at the end of lsof NAME fields like:
//   127.0.0.1:13542, *:3000, [::1]:8080
var portPattern = regexp.MustCompile(`:(\d+)\s*\(LISTEN\)`)

// parseLsofListening parses output from "lsof -iTCP -sTCP:LISTEN -P -n".
// Returns one entry per listening port/PID combination. Malformed lines are skipped.
func parseLsofListening(output string) ([]lsofEntry, error) {
	var entries []lsofEntry

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		nameField := fields[len(fields)-1] + " " + strings.Join(fields[len(fields)-1:], " ")
		// Look for the port pattern in the last two fields (NAME + state)
		tail := strings.Join(fields[8:], " ")
		match := portPattern.FindStringSubmatch(tail)
		if match == nil {
			continue
		}

		port, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		_ = nameField // suppress unused warning

		entries = append(entries, lsofEntry{
			PID:         pid,
			ProcessName: fields[0],
			Port:        port,
		})
	}

	return entries, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/portinfo/ -run TestParseLsofListening -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/portinfo/parse.go internal/portinfo/parse_test.go
git commit -m "feat(portinfo): add lsof listening port parser"
```

---

## Task 2: Parse ps Process Details Output

**Files:**
- Modify: `internal/portinfo/parse.go`
- Modify: `internal/portinfo/parse_test.go`

- [ ] **Step 1: Add ps parsing tests**

Append to `internal/portinfo/parse_test.go`:

```go
func TestParsePsOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[int]psEntry
	}{
		{
			name: "typical output",
			input: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
51002  1042 S   98304 Wed Mar 26 14:30:00 2026 ruby bin/rails server -p 3000
  412     1 Ss  25600 Mon Mar 24 08:00:00 2026 /usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Command: "node .next/standalone/server.js"},
				51002: {PID: 51002, PPID: 1042, State: "S", RSS: 98304, Command: "ruby bin/rails server -p 3000"},
				412:   {PID: 412, PPID: 1, State: "Ss", RSS: 25600, Command: "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main"},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int]psEntry{},
		},
		{
			name: "malformed line skipped",
			input: `not valid ps output
48291     1 S  142560 Thu Mar 27 09:15:00 2026 node server.js
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Command: "node server.js"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePsOutput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for pid, wantEntry := range tt.want {
				gotEntry, ok := got[pid]
				if !ok {
					t.Errorf("missing PID %d", pid)
					continue
				}
				if gotEntry.PID != wantEntry.PID || gotEntry.PPID != wantEntry.PPID ||
					gotEntry.State != wantEntry.State || gotEntry.RSS != wantEntry.RSS ||
					gotEntry.Command != wantEntry.Command {
					t.Errorf("PID %d: got %+v, want %+v", pid, gotEntry, wantEntry)
				}
				// StartTime is parsed from lstart — just verify it's not zero
				if gotEntry.StartTime.IsZero() {
					t.Errorf("PID %d: StartTime is zero", pid)
				}
			}
		})
	}
}

func TestParseLsofCwd(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[int]string
	}{
		{
			name: "typical output",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /Users/steve/src/myapp
ruby    51002 steve  cwd    DIR    1,4      320 5678 /Users/steve/src/railsapp
`,
			want: map[int]string{
				48291: "/Users/steve/src/myapp",
				51002: "/Users/steve/src/railsapp",
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLsofCwd(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for pid, wantCwd := range tt.want {
				if got[pid] != wantCwd {
					t.Errorf("PID %d: got %q, want %q", pid, got[pid], wantCwd)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portinfo/ -run "TestParsePsOutput|TestParseLsofCwd" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Add psEntry type and parsing functions to parse.go**

Append to `internal/portinfo/parse.go`:

```go
// psEntry holds parsed fields from a single ps output line.
type psEntry struct {
	PID       int
	PPID      int
	State     string
	RSS       int64 // kilobytes from ps, converted to bytes by caller
	StartTime time.Time
	Command   string
}

// parsePsOutput parses output from "ps -p ... -o pid=,ppid=,stat=,rss=,lstart=,command=".
// The lstart field is 5 tokens wide (e.g., "Thu Mar 27 09:15:00 2026").
// Returns a map of PID → psEntry. Malformed lines are skipped.
func parsePsOutput(output string) map[int]psEntry {
	entries := make(map[int]psEntry)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		// Minimum: pid, ppid, stat, rss, lstart (5 tokens), command (1+ tokens) = 9
		if len(fields) < 9 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		state := fields[2]
		rss, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			continue
		}

		// lstart is 5 tokens: "Thu Mar 27 09:15:00 2026"
		lstartStr := strings.Join(fields[4:9], " ")
		startTime, err := time.Parse("Mon Jan _2 15:04:05 2006", lstartStr)
		if err != nil {
			continue
		}

		command := strings.Join(fields[9:], " ")

		entries[pid] = psEntry{
			PID:       pid,
			PPID:      ppid,
			State:     state,
			RSS:       rss,
			StartTime: startTime,
			Command:   command,
		}
	}

	return entries
}

// parseLsofCwd parses output from "lsof -a -d cwd -p ...".
// Returns a map of PID → working directory path.
func parseLsofCwd(output string) map[int]string {
	cwds := make(map[int]string)

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		// NAME is the last field — the working directory path
		cwds[pid] = fields[len(fields)-1]
	}

	return cwds
}
```

Add `"time"` to the import block in `parse.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/portinfo/ -run "TestParsePsOutput|TestParseLsofCwd" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/portinfo/parse.go internal/portinfo/parse_test.go
git commit -m "feat(portinfo): add ps and lsof cwd parsers"
```

---

## Task 3: Framework and Orphan Detection

**Files:**
- Create: `internal/portinfo/detect.go`
- Create: `internal/portinfo/detect_test.go`

- [ ] **Step 1: Write detection tests**

```go
// internal/portinfo/detect_test.go
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
		wantProj  string
		wantFrame string
	}{
		{
			name: "Next.js from package.json",
			files: map[string]string{
				"package.json": `{"dependencies":{"next":"14.0.0"}}`,
			},
			wantProj:  "", // set dynamically from dir name
			wantFrame: "Next.js",
		},
		{
			name: "Rails from Gemfile",
			files: map[string]string{
				"Gemfile": `gem "rails", "~> 7.0"`,
			},
			wantProj:  "",
			wantFrame: "Rails",
		},
		{
			name: "Go from go.mod",
			files: map[string]string{
				"go.mod": `module github.com/example/myapp`,
			},
			wantProj:  "",
			wantFrame: "Go",
		},
		{
			name: "Nuxt from package.json",
			files: map[string]string{
				"package.json": `{"devDependencies":{"nuxt":"3.0.0"}}`,
			},
			wantProj:  "",
			wantFrame: "Nuxt",
		},
		{
			name: "Vite from package.json",
			files: map[string]string{
				"package.json": `{"devDependencies":{"vite":"5.0.0"}}`,
			},
			wantProj:  "",
			wantFrame: "Vite",
		},
		{
			name: "Django from manage.py",
			files: map[string]string{
				"manage.py": `#!/usr/bin/env python`,
			},
			wantProj:  "",
			wantFrame: "Django",
		},
		{
			name: "Rust from Cargo.toml",
			files: map[string]string{
				"Cargo.toml": `[package]`,
			},
			wantProj:  "",
			wantFrame: "Rust",
		},
		{
			name: "no markers returns empty",
			files: map[string]string{
				"readme.md": "hello",
			},
			wantProj:  "",
			wantFrame: "",
		},
		{
			name: "walks up from subdirectory",
			files: map[string]string{
				"package.json": `{"dependencies":{"express":"4.0.0"}}`,
			},
			wantProj:  "",
			wantFrame: "Express",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, content := range tt.files {
				fullPath := filepath.Join(dir, relPath)
				os.MkdirAll(filepath.Dir(fullPath), 0755)
				os.WriteFile(fullPath, []byte(content), 0644)
			}

			// For "walks up" test, use a subdirectory as CWD
			cwd := dir
			if tt.name == "walks up from subdirectory" {
				cwd = filepath.Join(dir, "src", "app")
				os.MkdirAll(cwd, 0755)
			}

			proj, frame := detectFramework(cwd)
			expectedProj := tt.wantProj
			if expectedProj == "" {
				expectedProj = filepath.Base(dir)
			}
			if tt.wantFrame == "" {
				expectedProj = "" // no marker found = no project
			}
			if proj != expectedProj {
				t.Errorf("project = %q, want %q", proj, expectedProj)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portinfo/ -run "TestDetectFramework|TestIsOrphan|TestIsZombie" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement detect.go**

```go
// internal/portinfo/detect.go
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
// Each function returns the detected framework name, or "" if the file
// doesn't reveal a specific framework.
var projectMarkers = []struct {
	file    string
	detecte func(dir string) string
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
				framework := marker.detecte(dir)
				return filepath.Base(dir), framework
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
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

	// Check both deps and devDeps, ordered by specificity
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
// for orphan detection. System daemons like postgres are excluded even if
// they have ppid=1, since that's normal for daemons.
var devProcessNames = map[string]bool{
	"node":    true,
	"ruby":    true,
	"python":  true,
	"python3": true,
	"go":      true,
	"cargo":   true,
	"deno":    true,
	"bun":     true,
	"java":    true,
	"php":     true,
	"elixir":  true,
	"beam.smp": true,
	"dotnet":  true,
}

// isOrphanProcess returns true when a process is likely an orphaned dev process:
// ppid=1 (adopted by init/launchd) AND the process name is a known dev runtime.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/portinfo/ -run "TestDetectFramework|TestIsOrphan|TestIsZombie" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/portinfo/detect.go internal/portinfo/detect_test.go
git commit -m "feat(portinfo): add framework and orphan detection"
```

---

## Task 4: Scanner Interface, Types, and Scan Functions

**Files:**
- Create: `internal/portinfo/portinfo.go`
- Create: `internal/portinfo/system.go`
- Create: `internal/portinfo/portinfo_test.go`

- [ ] **Step 1: Write Scan tests using a fake scanner**

```go
// internal/portinfo/portinfo_test.go
package portinfo

import (
	"testing"
)

// fakeScanner provides canned output for testing.
type fakeScanner struct {
	listeningOutput string
	listeningErr    error
	psOutput        string
	psErr           error
	cwdOutput       string
	cwdErr          error
}

func (f *fakeScanner) ListeningPorts() (string, error) {
	return f.listeningOutput, f.listeningErr
}

func (f *fakeScanner) ProcessInfo(pids []int) (string, error) {
	return f.psOutput, f.psErr
}

func (f *fakeScanner) WorkingDirs(pids []int) (string, error) {
	return f.cwdOutput, f.cwdErr
}

func TestScan(t *testing.T) {
	scanner := &fakeScanner{
		listeningOutput: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
`,
		psOutput: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
51002  1042 S   98304 Wed Mar 26 14:30:00 2026 ruby bin/rails server -p 3000
`,
		cwdOutput: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /tmp/fake-myapp
ruby    51002 steve  cwd    DIR    1,4      320 5678 /tmp/fake-railsapp
`,
	}

	results, err := Scan(scanner)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Verify first result (node)
	node := results[0]
	if node.Port != 13542 {
		t.Errorf("node port = %d, want 13542", node.Port)
	}
	if node.PID != 48291 {
		t.Errorf("node PID = %d, want 48291", node.PID)
	}
	if node.Name != "node" {
		t.Errorf("node name = %q, want %q", node.Name, "node")
	}
	if node.PPID != 1 {
		t.Errorf("node PPID = %d, want 1", node.PPID)
	}
	if node.RSS != 142560*1024 {
		t.Errorf("node RSS = %d, want %d", node.RSS, 142560*1024)
	}
	if node.IsOrphan != true {
		t.Error("node should be detected as orphan (ppid=1, dev process)")
	}
	if node.Command != "node .next/standalone/server.js" {
		t.Errorf("node command = %q, want %q", node.Command, "node .next/standalone/server.js")
	}

	// Verify second result (ruby)
	ruby := results[1]
	if ruby.Port != 3000 {
		t.Errorf("ruby port = %d, want 3000", ruby.Port)
	}
	if ruby.IsOrphan != false {
		t.Error("ruby should not be orphan (ppid=1042)")
	}
}

func TestScanPorts(t *testing.T) {
	scanner := &fakeScanner{
		listeningOutput: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
postgres    412  steve    5u  IPv4 0xaabbccddee      0t0  TCP 127.0.0.1:5432 (LISTEN)
`,
		psOutput: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
`,
		cwdOutput: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /tmp/fake-myapp
`,
	}

	results, err := ScanPorts([]int{13542}, scanner)
	if err != nil {
		t.Fatalf("ScanPorts() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Port != 13542 {
		t.Errorf("port = %d, want 13542", results[0].Port)
	}
}

func TestScan_EmptyOutput(t *testing.T) {
	scanner := &fakeScanner{
		listeningOutput: "",
	}

	results, err := Scan(scanner)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/portinfo/ -run "TestScan" -v`
Expected: FAIL — Scan/ScanPorts not defined

- [ ] **Step 3: Implement portinfo.go with types and Scan functions**

```go
// Package portinfo provides system-level port scanning and process inspection.
// It shells out to lsof and ps to discover listening TCP ports, identify the
// processes behind them, and detect orphaned or zombie dev processes.
//
// The Scanner interface abstracts the system calls so tests can inject canned
// output without hitting the real system.
package portinfo

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"
)

// ProcessInfo holds complete information about a process listening on a port.
type ProcessInfo struct {
	PID       int       `json:"pid"`
	PPID      int       `json:"ppid"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	Port      int       `json:"port"`
	RSS       int64     `json:"rss_bytes"`
	StartTime time.Time `json:"-"`
	State     string    `json:"-"`
	CWD       string    `json:"cwd,omitempty"`
	Project   string    `json:"project,omitempty"`
	Framework string    `json:"framework,omitempty"`
	IsOrphan  bool      `json:"is_orphan"`
	IsZombie  bool      `json:"is_zombie"`
}

// UptimeSeconds returns the process uptime as an integer for JSON output.
func (p ProcessInfo) UptimeSeconds() int64 {
	if p.StartTime.IsZero() {
		return 0
	}
	return int64(time.Since(p.StartTime).Seconds())
}

// Scanner abstracts the system commands used for port discovery.
// The real implementation (SystemScanner) shells out to lsof and ps.
// Tests inject a fake with canned output.
type Scanner interface {
	ListeningPorts() (string, error)
	ProcessInfo(pids []int) (string, error)
	WorkingDirs(pids []int) (string, error)
}

// Scan discovers all listening TCP ports and returns enriched process info.
func Scan(scanner Scanner) ([]ProcessInfo, error) {
	return scan(scanner, nil)
}

// ScanPorts scans only the specified ports. Used for project-scoped views
// where we only care about Outport-managed ports.
func ScanPorts(ports []int, scanner Scanner) ([]ProcessInfo, error) {
	filter := make(map[int]bool, len(ports))
	for _, p := range ports {
		filter[p] = true
	}
	return scan(scanner, filter)
}

func scan(scanner Scanner, portFilter map[int]bool) ([]ProcessInfo, error) {
	lsofOutput, err := scanner.ListeningPorts()
	if err != nil {
		return nil, fmt.Errorf("listing ports: %w", err)
	}

	entries, err := parseLsofListening(lsofOutput)
	if err != nil {
		return nil, fmt.Errorf("parsing lsof: %w", err)
	}

	// Filter to requested ports if specified
	if portFilter != nil {
		var filtered []lsofEntry
		for _, e := range entries {
			if portFilter[e.Port] {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Collect unique PIDs
	pidSet := make(map[int]bool)
	for _, e := range entries {
		pidSet[e.PID] = true
	}
	pids := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}

	// Batch: get process details
	psOutput, err := scanner.ProcessInfo(pids)
	if err != nil {
		return nil, fmt.Errorf("getting process info: %w", err)
	}
	psEntries := parsePsOutput(psOutput)

	// Batch: get working directories
	cwdOutput, err := scanner.WorkingDirs(pids)
	if err != nil {
		// CWD lookup failure is non-fatal — we still have port + process info
		cwdOutput = ""
	}
	cwds := parseLsofCwd(cwdOutput)

	// Build results
	results := make([]ProcessInfo, 0, len(entries))
	for _, entry := range entries {
		info := ProcessInfo{
			PID:  entry.PID,
			Name: entry.ProcessName,
			Port: entry.Port,
		}

		if ps, ok := psEntries[entry.PID]; ok {
			info.PPID = ps.PPID
			info.State = ps.State
			info.RSS = ps.RSS * 1024 // ps reports KB, we store bytes
			info.StartTime = ps.StartTime
			info.Command = ps.Command
			info.IsOrphan = isOrphanProcess(ps.PPID, entry.ProcessName)
			info.IsZombie = isZombieProcess(ps.State)
		}

		if cwd, ok := cwds[entry.PID]; ok {
			info.CWD = cwd
			info.Project, info.Framework = detectFramework(cwd)
		}

		results = append(results, info)
	}

	// Sort by port for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	return results, nil
}

// Kill sends SIGTERM to the given PID. Returns an error for protected PIDs
// (0, 1) or if the signal fails.
func Kill(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to kill PID %d", pid)
	}
	// Refuse to kill the outport daemon itself
	if pid == os.Getpid() {
		return fmt.Errorf("refusing to kill outport's own process")
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
```

- [ ] **Step 4: Implement SystemScanner in system.go**

```go
// internal/portinfo/system.go
package portinfo

import (
	"fmt"
	"os/exec"
	"strings"
)

// SystemScanner implements Scanner by shelling out to real system commands.
type SystemScanner struct{}

// ListeningPorts runs lsof to find all listening TCP ports.
func (s SystemScanner) ListeningPorts() (string, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n").Output()
	if err != nil {
		// lsof returns exit code 1 when no results found — that's not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("lsof: %w", err)
	}
	return string(out), nil
}

// ProcessInfo runs ps to get details for the given PIDs.
func (s SystemScanner) ProcessInfo(pids []int) (string, error) {
	if len(pids) == 0 {
		return "", nil
	}
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = fmt.Sprintf("%d", pid)
	}
	out, err := exec.Command("ps", "-p", strings.Join(pidStrs, ","),
		"-o", "pid=,ppid=,stat=,rss=,lstart=,command=").Output()
	if err != nil {
		return "", fmt.Errorf("ps: %w", err)
	}
	return string(out), nil
}

// WorkingDirs runs lsof to get the working directory for the given PIDs.
func (s SystemScanner) WorkingDirs(pids []int) (string, error) {
	if len(pids) == 0 {
		return "", nil
	}
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = fmt.Sprintf("%d", pid)
	}
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(pidStrs, ",")).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("lsof cwd: %w", err)
	}
	return string(out), nil
}
```

- [ ] **Step 5: Run all portinfo tests**

Run: `go test ./internal/portinfo/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/portinfo/portinfo.go internal/portinfo/system.go internal/portinfo/portinfo_test.go
git commit -m "feat(portinfo): add Scanner interface, Scan, ScanPorts, Kill, and SystemScanner"
```

---

## Task 5: `outport ports` Command — Project-Scoped View

**Files:**
- Create: `cmd/ports.go`
- Create: `cmd/ports_test.go`

- [ ] **Step 1: Write tests for the ports command**

```go
// cmd/ports_test.go
package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestPortsCommand_JSONFlag(t *testing.T) {
	// Verify the command exists and accepts --json
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ports", "--json", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("ports --json --help failed: %v", err)
	}
}

func TestPortsCommand_AllFlag(t *testing.T) {
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"ports", "--all", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("ports --all --help failed: %v", err)
	}
}

func TestPortsCommand_HasKillSubcommand(t *testing.T) {
	var found bool
	for _, sub := range portsCmd.Commands() {
		if sub.Name() == "kill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ports command should have a 'kill' subcommand")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestPortsCommand" -v`
Expected: FAIL — portsCmd not defined

- [ ] **Step 3: Implement ports.go**

```go
// cmd/ports.go
package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/portinfo"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/steveclarke/outport/internal/urlutil"
	"github.com/spf13/cobra"
)

var portsAllFlag bool

var portsCmd = &cobra.Command{
	Use:     "ports",
	Short:   "Show live port and process information",
	Long:    "Shows allocated ports with live process details (PID, memory, uptime, framework). Inside a project directory, shows this project's ports. Outside, shows all Outport-managed ports. Use --all for a full machine scan.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runPorts,
}

func init() {
	portsCmd.Flags().BoolVar(&portsAllFlag, "all", false, "scan all listening ports on the machine")
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	scanner := portinfo.SystemScanner{}

	if portsAllFlag {
		return runPortsAll(cmd, scanner)
	}

	// Try project-scoped view first
	ctx, err := loadProjectContext()
	if err != nil {
		// Outside a project — show all Outport-managed ports
		return runPortsAllOutport(cmd, scanner)
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	ports := slices.Collect(maps.Values(alloc.Ports))
	processes, err := portinfo.ScanPorts(ports, scanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}

	processByPort := indexByPort(processes)
	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsProjectJSON(cmd, ctx.Cfg, ctx.Instance, alloc, processByPort, httpsEnabled)
	}
	return printPortsProjectStyled(cmd, ctx.Cfg, ctx.Instance, alloc, processByPort, httpsEnabled)
}

func runPortsAllOutport(cmd *cobra.Command, scanner portinfo.Scanner) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	projects := reg.All()
	if len(projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport up' in a project directory.")
		return nil
	}

	// Collect all Outport-managed ports
	var allPorts []int
	for _, alloc := range projects {
		for _, port := range alloc.Ports {
			allPorts = append(allPorts, port)
		}
	}

	processes, err := portinfo.ScanPorts(allPorts, scanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}
	processByPort := indexByPort(processes)
	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsAllOutportJSON(cmd, reg, projects, processByPort, httpsEnabled)
	}
	return printPortsAllOutportStyled(cmd, reg, projects, processByPort, httpsEnabled)
}

func runPortsAll(cmd *cobra.Command, scanner portinfo.Scanner) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	processes, err := portinfo.Scan(scanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}

	// Build managed port set from registry
	managedPorts := make(map[int]managedPort)
	for key, alloc := range reg.All() {
		for svcName, port := range alloc.Ports {
			managedPorts[port] = managedPort{
				RegistryKey: key,
				ServiceName: svcName,
				Hostname:    alloc.Hostnames[svcName],
			}
		}
	}

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsAllJSON(cmd, processes, managedPorts, httpsEnabled)
	}
	return printPortsAllStyled(cmd, processes, managedPorts, httpsEnabled)
}

type managedPort struct {
	RegistryKey string
	ServiceName string
	Hostname    string
}

func indexByPort(processes []portinfo.ProcessInfo) map[int]portinfo.ProcessInfo {
	m := make(map[int]portinfo.ProcessInfo, len(processes))
	for _, p := range processes {
		m[p.Port] = p
	}
	return m
}

// --- JSON output ---

type portProcessJSON struct {
	PID           int    `json:"pid"`
	PPID          int    `json:"ppid"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	RSSBytes      int64  `json:"rss_bytes"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	CWD           string `json:"cwd,omitempty"`
	Project       string `json:"project,omitempty"`
	Framework     string `json:"framework,omitempty"`
	IsOrphan      bool   `json:"is_orphan"`
	IsZombie      bool   `json:"is_zombie"`
}

func toProcessJSON(p portinfo.ProcessInfo) *portProcessJSON {
	return &portProcessJSON{
		PID:           p.PID,
		PPID:          p.PPID,
		Name:          p.Name,
		Command:       p.Command,
		RSSBytes:      p.RSS,
		UptimeSeconds: p.UptimeSeconds(),
		CWD:           p.CWD,
		Project:       p.Project,
		Framework:     p.Framework,
		IsOrphan:      p.IsOrphan,
		IsZombie:      p.IsZombie,
	}
}

type portEntryJSON struct {
	Port        int              `json:"port"`
	Service     string           `json:"service,omitempty"`
	RegistryKey string           `json:"registry_key,omitempty"`
	Hostname    string           `json:"hostname,omitempty"`
	URL         string           `json:"url,omitempty"`
	Up          bool             `json:"up"`
	Process     *portProcessJSON `json:"process,omitempty"`
}

type portsProjectJSON struct {
	Project  string          `json:"project"`
	Instance string          `json:"instance"`
	Ports    []portEntryJSON `json:"ports"`
}

func printPortsProjectJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, processByPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	var entries []portEntryJSON
	svcNames := slices.Sorted(maps.Keys(alloc.Ports))
	for _, svcName := range svcNames {
		port := alloc.Ports[svcName]
		hostname := alloc.Hostnames[svcName]
		entry := portEntryJSON{
			Port:     port,
			Service:  svcName,
			Hostname: hostname,
			URL:      urlutil.ServiceURL(hostname, port, httpsEnabled),
		}
		if p, ok := processByPort[port]; ok {
			entry.Up = true
			entry.Process = toProcessJSON(p)
		}
		entries = append(entries, entry)
	}

	upCount := 0
	for _, e := range entries {
		if e.Up {
			upCount++
		}
	}

	out := portsProjectJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Ports:    entries,
	}
	summary := fmt.Sprintf("%d %s, %d up", len(entries), pluralize(len(entries), "port", "ports"), upCount)
	return writeJSON(cmd, out, summary)
}

type portsAllJSON struct {
	Managed []portEntryJSON `json:"managed"`
	Other   []portEntryJSON `json:"other,omitempty"`
}

func printPortsAllJSON(cmd *cobra.Command, processes []portinfo.ProcessInfo, managed map[int]managedPort, httpsEnabled bool) error {
	var managedEntries, otherEntries []portEntryJSON
	for _, p := range processes {
		if mp, ok := managed[p.Port]; ok {
			entry := portEntryJSON{
				Port:        p.Port,
				Service:     mp.ServiceName,
				RegistryKey: mp.RegistryKey,
				Hostname:    mp.Hostname,
				URL:         urlutil.ServiceURL(mp.Hostname, p.Port, httpsEnabled),
				Up:          true,
				Process:     toProcessJSON(p),
			}
			managedEntries = append(managedEntries, entry)
		} else {
			entry := portEntryJSON{
				Port:    p.Port,
				Up:      true,
				Process: toProcessJSON(p),
			}
			otherEntries = append(otherEntries, entry)
		}
	}

	// Add managed ports that are down (not in scan results)
	scannedPorts := make(map[int]bool)
	for _, p := range processes {
		scannedPorts[p.Port] = true
	}
	for port, mp := range managed {
		if !scannedPorts[port] {
			managedEntries = append(managedEntries, portEntryJSON{
				Port:        port,
				Service:     mp.ServiceName,
				RegistryKey: mp.RegistryKey,
				Hostname:    mp.Hostname,
				URL:         urlutil.ServiceURL(mp.Hostname, port, httpsEnabled),
				Up:          false,
			})
		}
	}

	out := portsAllJSON{Managed: managedEntries, Other: otherEntries}
	mCount := len(managedEntries)
	oCount := len(otherEntries)
	upCount := 0
	for _, e := range managedEntries {
		if e.Up {
			upCount++
		}
	}
	upCount += len(otherEntries) // all scanned "other" are up by definition
	total := mCount + oCount
	summary := fmt.Sprintf("%d %s (%d managed, %d other), %d up", total, pluralize(total, "port", "ports"), mCount, oCount, upCount)
	return writeJSON(cmd, out, summary)
}

func printPortsAllOutportJSON(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, processByPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	var entries []portEntryJSON
	keys := slices.Sorted(maps.Keys(projects))
	for _, key := range keys {
		alloc := projects[key]
		svcNames := slices.Sorted(maps.Keys(alloc.Ports))
		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]
			hostname := alloc.Hostnames[svcName]
			entry := portEntryJSON{
				Port:        port,
				Service:     svcName,
				RegistryKey: key,
				Hostname:    hostname,
				URL:         urlutil.ServiceURL(hostname, port, httpsEnabled),
			}
			if p, ok := processByPort[port]; ok {
				entry.Up = true
				entry.Process = toProcessJSON(p)
			}
			entries = append(entries, entry)
		}
	}

	upCount := 0
	for _, e := range entries {
		if e.Up {
			upCount++
		}
	}
	summary := fmt.Sprintf("%d %s, %d up", len(entries), pluralize(len(entries), "port", "ports"), upCount)
	return writeJSON(cmd, portsAllJSON{Managed: entries}, summary)
}

// --- Styled output ---

func printPortsProjectStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, processByPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, instanceName)

	svcNames := slices.Sorted(maps.Keys(alloc.Ports))
	for _, svcName := range svcNames {
		port := alloc.Ports[svcName]
		up := false
		if _, ok := processByPort[port]; ok {
			up = true
		}
		portStatus := map[int]bool{port: up}
		printServiceLineDetailed(w, cfg, svcName, port, alloc.Hostnames, portStatus, httpsEnabled)
		if svcAliases, ok := alloc.Aliases[svcName]; ok {
			printAliasLines(w, svcAliases, port, httpsEnabled)
		}
		if p, ok := processByPort[port]; ok {
			printProcessLine(w, p)
		}
	}

	return nil
}

func printPortsAllOutportStyled(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, processByPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey(reg)
	keys := slices.Sorted(maps.Keys(projects))

	for i, key := range keys {
		alloc := projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)

		marker := ""
		if key == currentKey {
			marker = currentMarker.Render(" ●")
		}
		displayName := formatProjectKey(key)
		header := ui.ProjectStyle.Render(displayName) + " " + ui.DimStyle.Render(alloc.ProjectDir) + marker
		lipgloss.Fprintln(w, header)

		svcNames := slices.Sorted(maps.Keys(alloc.Ports))
		renderCfg := cfg
		if renderCfg == nil {
			renderCfg = &config.Config{Services: make(map[string]config.Service)}
		}
		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]
			up := false
			if _, ok := processByPort[port]; ok {
				up = true
			}
			portStatus := map[int]bool{port: up}
			printServiceLineCompact(w, renderCfg, svcName, port, alloc.Hostnames, portStatus, httpsEnabled)
			if p, ok := processByPort[port]; ok {
				printProcessLine(w, p)
			}
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	return nil
}

func printPortsAllStyled(cmd *cobra.Command, processes []portinfo.ProcessInfo, managed map[int]managedPort, httpsEnabled bool) error {
	w := cmd.OutOrStdout()

	// Split into managed and other
	var managedProcs, otherProcs []portinfo.ProcessInfo
	for _, p := range processes {
		if _, ok := managed[p.Port]; ok {
			managedProcs = append(managedProcs, p)
		} else {
			otherProcs = append(otherProcs, p)
		}
	}

	if len(managedProcs) > 0 || len(managed) > 0 {
		lipgloss.Fprintln(w, ui.ProjectStyle.Render("Outport managed"))
		for _, p := range managedProcs {
			mp := managed[p.Port]
			printManagedPortLine(w, p, mp, httpsEnabled)
		}
		// Show down managed ports too
		scannedPorts := make(map[int]bool)
		for _, p := range processes {
			scannedPorts[p.Port] = true
		}
		for port, mp := range managed {
			if !scannedPorts[port] {
				line := fmt.Sprintf("    %s  %s %-5s  %s",
					ui.ServiceStyle.Render(fmt.Sprintf("%-20s", mp.RegistryKey+"/"+mp.ServiceName)),
					ui.Arrow,
					ui.PortStyle.Render(fmt.Sprintf("%d", port)),
					ui.StatusDown,
				)
				lipgloss.Fprintln(w, line)
			}
		}
	}

	if len(otherProcs) > 0 {
		if len(managedProcs) > 0 || len(managed) > 0 {
			lipgloss.Fprintln(w)
		}
		lipgloss.Fprintln(w, ui.ProjectStyle.Render("Other"))
		for _, p := range otherProcs {
			printUnmanagedPortLine(w, p)
		}
	}

	return nil
}

func printManagedPortLine(w io.Writer, p portinfo.ProcessInfo, mp managedPort, httpsEnabled bool) {
	urlStr := ""
	if u := urlutil.ServiceURL(mp.Hostname, p.Port, httpsEnabled); u != "" {
		urlStr = "  " + ui.UrlStyle.Render(u)
	}
	line := fmt.Sprintf("    %s  %s %-5s  %s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-20s", mp.RegistryKey+"/"+mp.ServiceName)),
		ui.Arrow,
		ui.PortStyle.Render(fmt.Sprintf("%d", p.Port)),
		ui.StatusUp,
		urlStr,
	)
	lipgloss.Fprintln(w, line)
	printProcessLine(w, p)
}

func printUnmanagedPortLine(w io.Writer, p portinfo.ProcessInfo) {
	framework := ""
	if p.Framework != "" {
		framework = " (" + p.Framework + ")"
	}
	orphanMarker := ""
	if p.IsOrphan {
		orphanMarker = "  " + ui.WarnStyle.Render("⚠ orphan")
	}
	if p.IsZombie {
		orphanMarker = "  " + ui.WarnStyle.Render("⚠ zombie")
	}
	line := fmt.Sprintf("    %-8s%s%s",
		ui.PortStyle.Render(fmt.Sprintf("%d", p.Port)),
		ui.ServiceStyle.Render(p.Name+framework),
		orphanMarker,
	)
	lipgloss.Fprintln(w, line)
	printProcessLine(w, p)
}

func printProcessLine(w io.Writer, p portinfo.ProcessInfo) {
	parts := []string{fmt.Sprintf("PID %d", p.PID)}

	// Command — show a short version
	cmd := p.Command
	if p.Name != "" && cmd != "" {
		parts[0] = fmt.Sprintf("PID %d · %s", p.PID, truncate(cmd, 40))
	}

	if p.RSS > 0 {
		parts = append(parts, formatMemory(p.RSS))
	}
	if !p.StartTime.IsZero() {
		parts = append(parts, formatUptime(time.Since(p.StartTime)))
	}

	line := fmt.Sprintf("    %s",
		ui.DimStyle.Render(fmt.Sprintf("%-38s", "")+"  "+joinParts(parts)))
	lipgloss.Fprintln(w, line)
}

func formatMemory(bytes int64) string {
	mb := bytes / (1024 * 1024)
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
	return fmt.Sprintf("%d MB", mb)
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours()) / 24
	return fmt.Sprintf("%dd", days)
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " · "
		}
		result += p
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestPortsCommand" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `just test`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/ports.go cmd/ports_test.go
git commit -m "feat: add 'outport ports' command with live process info"
```

---

## Task 6: `outport ports kill` Subcommand

**Files:**
- Create: `cmd/ports_kill.go`
- Create: `cmd/ports_kill_test.go`

- [ ] **Step 1: Write kill command tests**

```go
// cmd/ports_kill_test.go
package cmd

import (
	"testing"
)

func TestPortsKillCommand_RequiresTarget(t *testing.T) {
	cmd := rootCmd
	cmd.SetArgs([]string{"ports", "kill"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no target provided")
	}
}

func TestPortsKillCommand_HasOrphansFlag(t *testing.T) {
	killCmd := portsKillCmd
	flag := killCmd.Flags().Lookup("orphans")
	if flag == nil {
		t.Error("kill command should have --orphans flag")
	}
}

func TestPortsKillCommand_HasForceFlag(t *testing.T) {
	killCmd := portsKillCmd
	flag := killCmd.Flags().Lookup("force")
	if flag == nil {
		t.Error("kill command should have --force flag")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run "TestPortsKillCommand" -v`
Expected: FAIL — portsKillCmd not defined

- [ ] **Step 3: Implement ports_kill.go**

```go
// cmd/ports_kill.go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/portinfo"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var (
	killOrphansFlag bool
	killForceFlag   bool
)

var portsKillCmd = &cobra.Command{
	Use:   "kill <service-or-port>",
	Short: "Kill the process on a port",
	Long:  "Kills the process listening on a port. Target can be a service name (when inside a project) or a port number. Use --orphans to kill all orphaned dev processes.",
	Args: func(cmd *cobra.Command, args []string) error {
		if killOrphansFlag {
			if len(args) > 0 {
				return FlagErrorf("--orphans does not accept arguments")
			}
			return nil
		}
		if len(args) == 0 {
			return FlagErrorf("requires a service name or port number")
		}
		if len(args) > 1 {
			return FlagErrorf("too many arguments")
		}
		return nil
	},
	RunE: runPortsKill,
}

func init() {
	portsKillCmd.Flags().BoolVar(&killOrphansFlag, "orphans", false, "kill all orphaned dev processes")
	portsKillCmd.Flags().BoolVar(&killForceFlag, "force", false, "skip confirmation prompt")
	portsCmd.AddCommand(portsKillCmd)
}

// killTarget resolves a service name or port number to a port.
func killTarget(target string) (int, error) {
	// Try as port number first
	if port, err := strconv.Atoi(target); err == nil {
		if port < 1 || port > 65535 {
			return 0, fmt.Errorf("port %d out of range (1-65535)", port)
		}
		return port, nil
	}

	// Try as service name (requires project context)
	ctx, err := loadProjectContext()
	if err != nil {
		return 0, fmt.Errorf("service name %q requires a project context (cd into a project dir, or use a port number)", target)
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return 0, fmt.Errorf("project not registered — run 'outport up' first")
	}

	port, ok := alloc.Ports[target]
	if !ok {
		return 0, fmt.Errorf("service %q not found in this project", target)
	}

	return port, nil
}

func runPortsKill(cmd *cobra.Command, args []string) error {
	scanner := portinfo.SystemScanner{}

	if killOrphansFlag {
		return runKillOrphans(cmd, scanner)
	}

	port, err := killTarget(args[0])
	if err != nil {
		return err
	}

	processes, err := portinfo.ScanPorts([]int{port}, scanner)
	if err != nil {
		return fmt.Errorf("scanning port %d: %w", port, err)
	}

	if len(processes) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No process listening on port %d.\n", port)
		return nil
	}

	// Multiple PIDs on the same port: pick the lowest PID (likely the parent).
	// Show all to the user so they know what's being killed.
	p := processes[0]
	for _, proc := range processes[1:] {
		if proc.PID < p.PID {
			p = proc
		}
	}

	if len(processes) > 1 {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "%d processes on port %d — targeting parent PID %d:\n",
			len(processes), port, p.PID)
		for _, proc := range processes {
			marker := "  "
			if proc.PID == p.PID {
				marker = "→ "
			}
			fmt.Fprintf(w, "  %sPID %d · %s\n", marker, proc.PID, truncate(proc.Command, 50))
		}
		fmt.Fprintln(w)
	}

	if jsonFlag {
		return printKillJSON(cmd, p, port)
	}

	return killWithConfirmation(cmd, p, port)
}

func runKillOrphans(cmd *cobra.Command, scanner portinfo.Scanner) error {
	processes, err := portinfo.Scan(scanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}

	var orphans []portinfo.ProcessInfo
	for _, p := range processes {
		if p.IsOrphan || p.IsZombie {
			orphans = append(orphans, p)
		}
	}

	if len(orphans) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No orphaned processes found.")
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Found %d orphaned %s:\n\n", len(orphans), pluralize(len(orphans), "process", "processes"))
	for _, p := range orphans {
		printKillCandidate(w, p)
	}

	if !killForceFlag {
		fmt.Fprintf(w, "\nKill all? [y/N]: ")
		if !confirmYes(os.Stdin) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}

	var killed, failed int
	for _, p := range orphans {
		if err := portinfo.Kill(p.PID); err != nil {
			fmt.Fprintf(w, "%s Failed to kill PID %d: %v\n",
				lipgloss.NewStyle().Foreground(ui.Red).Render("✗"), p.PID, err)
			failed++
		} else {
			fmt.Fprintf(w, "%s Killed PID %d (SIGTERM)\n",
				lipgloss.NewStyle().Foreground(ui.Green).Render("✓"), p.PID)
			killed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(w, "\n%d killed, %d failed. For stubborn processes, try: sudo kill -9 <pid>\n", killed, failed)
	}

	return nil
}

func killWithConfirmation(cmd *cobra.Command, p portinfo.ProcessInfo, port int) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Kill process on port %s?\n", ui.PortStyle.Render(fmt.Sprintf("%d", port)))
	printKillCandidate(w, p)

	if !killForceFlag {
		fmt.Fprintf(w, "\nConfirm [y/N]: ")
		if !confirmYes(os.Stdin) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}

	if err := portinfo.Kill(p.PID); err != nil {
		fmt.Fprintf(w, "%s Failed to kill PID %d: %v\n",
			lipgloss.NewStyle().Foreground(ui.Red).Render("✗"), p.PID, err)
		fmt.Fprintf(w, "Try: sudo kill -9 %d\n", p.PID)
		return ErrSilent
	}

	fmt.Fprintf(w, "%s Killed PID %d (SIGTERM)\n",
		lipgloss.NewStyle().Foreground(ui.Green).Render("✓"), p.PID)

	// Check if still running after a brief wait
	time.Sleep(500 * time.Millisecond)
	check, _ := portinfo.ScanPorts([]int{port}, portinfo.SystemScanner{})
	if len(check) > 0 {
		fmt.Fprintf(w, "\nProcess still running. Try: sudo kill -9 %d\n", p.PID)
	}

	return nil
}

func printKillCandidate(w io.Writer, p portinfo.ProcessInfo) {
	parts := []string{fmt.Sprintf("PID %d", p.PID)}
	if p.Command != "" {
		parts = append(parts, truncate(p.Command, 40))
	}
	if p.Project != "" {
		parts = append(parts, p.Project)
	}
	if p.RSS > 0 {
		parts = append(parts, formatMemory(p.RSS))
	}
	if !p.StartTime.IsZero() {
		parts = append(parts, formatUptime(time.Since(p.StartTime)))
	}
	fmt.Fprintf(w, "  %s\n", ui.DimStyle.Render(joinParts(parts)))
}

type killJSON struct {
	Port    int              `json:"port"`
	Process *portProcessJSON `json:"process"`
	Killed  bool             `json:"killed"`
}

func printKillJSON(cmd *cobra.Command, p portinfo.ProcessInfo, port int) error {
	if !killForceFlag {
		return fmt.Errorf("--json requires --force (no interactive prompt in JSON mode)")
	}

	killed := true
	if err := portinfo.Kill(p.PID); err != nil {
		killed = false
	}

	out := killJSON{
		Port:    port,
		Process: toProcessJSON(p),
		Killed:  killed,
	}
	action := "killed"
	if !killed {
		action = "failed"
	}
	return writeJSON(cmd, out, action)
}

// confirmYes reads a line from stdin and returns true if it starts with 'y' or 'Y'.
func confirmYes(r io.Reader) bool {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}
	line := strings.TrimSpace(scanner.Text())
	return strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run "TestPortsKillCommand" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `just test`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/ports_kill.go cmd/ports_kill_test.go
git commit -m "feat: add 'outport ports kill' subcommand"
```

---

## Task 7: Doctor Integration — Orphan Check

**Files:**
- Modify: `internal/doctor/project.go`
- Modify: `internal/doctor/project_test.go`

- [ ] **Step 1: Write test for orphan check in doctor**

Add to `internal/doctor/project_test.go`. First read the existing file to see the test patterns, then append:

```go
func TestProjectChecks_IncludesOrphanCheck(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal outport.yml
	cfgContent := `name: testproject
services:
  web:
    env_var: PORT
`
	os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(cfgContent), 0644)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// Create a registry with an allocation for this project
	regPath := filepath.Join(t.TempDir(), "registry.json")
	reg := &registry.Registry{Projects: map[string]registry.Allocation{
		"testproject/main": {
			ProjectDir: dir,
			Ports:      map[string]int{"web": 19999},
			Hostnames:  map[string]string{},
		},
	}}
	data, _ := json.MarshalIndent(reg, "", "  ")
	os.WriteFile(regPath, data, 0644)

	checks := ProjectChecks(dir, cfg, nil, regPath)

	var foundOrphan bool
	for _, c := range checks {
		if c.Name == "Orphaned processes" {
			foundOrphan = true
			break
		}
	}
	if !foundOrphan {
		t.Error("ProjectChecks should include an 'Orphaned processes' check")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/ -run TestProjectChecks_IncludesOrphanCheck -v`
Expected: FAIL — no "Orphaned processes" check exists yet

- [ ] **Step 3: Add orphan check to ProjectChecks in project.go**

Add to the bottom of `ProjectChecks()` in `internal/doctor/project.go`, after the port checks loop, still inside the `if found {` block:

```go
		// Orphan check — scan managed ports for orphaned/zombie processes
		allPorts := make([]int, 0, len(alloc.Ports))
		for _, port := range alloc.Ports {
			allPorts = append(allPorts, port)
		}
		checks = append(checks, Check{
			Name:     "Orphaned processes",
			Category: category,
			Run: func() *Result {
				processes, err := portinfo.ScanPorts(allPorts, portinfo.SystemScanner{})
				if err != nil {
					return &Result{
						Name:    "Orphaned processes",
						Status:  Warn,
						Message: fmt.Sprintf("could not scan ports: %v", err),
					}
				}
				var orphanPorts []string
				for _, p := range processes {
					if p.IsOrphan || p.IsZombie {
						orphanPorts = append(orphanPorts, fmt.Sprintf("%d (%s)", p.Port, p.Name))
					}
				}
				if len(orphanPorts) > 0 {
					return &Result{
						Name:    "Orphaned processes",
						Status:  Warn,
						Message: fmt.Sprintf("orphaned processes on: %s", strings.Join(orphanPorts, ", ")),
						Fix:     "Run: outport ports kill --orphans",
					}
				}
				return &Result{
					Name:    "Orphaned processes",
					Status:  Pass,
					Message: "no orphaned processes on managed ports",
				}
			},
		})
```

Add imports for `"strings"` and `"github.com/steveclarke/outport/internal/portinfo"` to `project.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/doctor/ -run TestProjectChecks_IncludesOrphanCheck -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `just test`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/doctor/project.go internal/doctor/project_test.go
git commit -m "feat(doctor): add orphaned process detection to project checks"
```

---

## Task 8: Lint, Full Test Suite, and Cleanup

**Files:**
- Possibly modify: any files with lint warnings

- [ ] **Step 1: Run linter**

Run: `just lint`
Expected: PASS (no warnings). If there are warnings, fix them.

- [ ] **Step 2: Run full test suite**

Run: `just test`
Expected: ALL PASS

- [ ] **Step 3: Run security scanner**

Run: `just gosec`
Expected: PASS

- [ ] **Step 4: Verify --json output manually**

Run: `go run . ports --json` (from an Outport project directory)
Verify: Valid JSON with `{"ok": true, "data": {...}, "summary": "..."}` envelope.

Run: `go run . ports --all --json`
Verify: Valid JSON with `managed` and `other` arrays.

- [ ] **Step 5: Verify styled output manually**

Run: `go run . ports` (from an Outport project directory)
Verify: Shows project header, service lines with process details underneath.

Run: `go run . ports --all`
Verify: Shows "Outport managed" and "Other" sections.

- [ ] **Step 6: Commit any cleanup**

```bash
git add -A
git commit -m "chore: lint and cleanup for ports command"
```

---

## Task 9: Update CLAUDE.md and Docs

**Files:**
- Modify: `CLAUDE.md`
- Possibly modify: docs site if commands list needs updating

- [ ] **Step 1: Update CLAUDE.md**

Add `portinfo` to the core packages list in the Architecture section:

```
- **portinfo** — System-level port scanning and process inspection. Shells out to `lsof`/`ps` to discover listening TCP ports, identify processes, detect frameworks, and flag orphaned/zombie processes. `Scanner` interface enables test injection.
```

Add to CLI commands section a note that `ports` and `ports kill` exist.

- [ ] **Step 2: Verify README commands list matches**

Check that the README's command list includes `ports` and `ports kill`.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: add portinfo package and ports command to docs"
```

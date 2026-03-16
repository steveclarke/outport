# Local DNS + Reverse Proxy for .test Domains — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give HTTP services friendly `.test` hostnames via local DNS + reverse proxy, with automatic cookie isolation across parallel instances.

**Architecture:** Extend the registry to store hostnames and protocols. Replace worktree detection with registry-based instance resolution. Add an embedded DNS server (miekg/dns) and HTTP reverse proxy as a daemon process, managed via macOS launchd socket activation. Template system gains a `:modifier` syntax for context-dependent URL generation.

**Tech Stack:** Go 1.26, miekg/dns, fsnotify/fsnotify, macOS launchd (cgo for launch_activate_socket), httputil.ReverseProxy

**Spec:** `docs/superpowers/specs/2026-03-15-test-domains-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/instance/instance.go` | Instance resolution: resolve current instance from registry, generate 4-char codes, validate instance names |
| `internal/instance/instance_test.go` | Tests for instance resolution and code generation |
| `internal/daemon/dns.go` | Embedded DNS server using miekg/dns — answers `*.test` → 127.0.0.1 |
| `internal/daemon/dns_test.go` | Tests for DNS server |
| `internal/daemon/proxy.go` | HTTP reverse proxy with WebSocket support, route table management |
| `internal/daemon/proxy_test.go` | Tests for proxy routing, WebSocket upgrade detection, error pages |
| `internal/daemon/routes.go` | Route table builder: reads registry, builds hostname→port map, fsnotify watcher |
| `internal/daemon/routes_test.go` | Tests for route table building and registry watching |
| `internal/daemon/daemon.go` | Daemon entry point: starts DNS + proxy servers, coordinates shutdown |
| `internal/daemon/daemon_test.go` | Integration tests for daemon startup/shutdown |
| `internal/platform/platform.go` | Platform interface: `Setup()`, `Teardown()`, `IsSetup()`, `LoadListenerFromLaunchd()` |
| `internal/platform/darwin.go` | macOS implementation: resolver file, LaunchAgent plist, launchd socket activation |
| `internal/platform/darwin_test.go` | Tests for macOS platform operations (where testable without root) |
| `cmd/setup.go` | `outport setup` and `outport teardown` commands |
| `cmd/updown.go` | `outport up` and `outport down` commands |
| `cmd/rename.go` | `outport rename <old> <new>` command |
| `cmd/promote.go` | `outport promote` command |

### Modified Files

| File | Changes |
|------|---------|
| `internal/registry/registry.go` | Extend `Allocation` struct with `Hostnames` and `Protocols` fields. Add `FindByDir()`, `FindByProject()` methods. |
| `internal/config/config.go` | Extend template regex for `:modifier` syntax. Add `url` to `validFields`. Add hostname validation (DNS-safe chars, must contain project name). Validate `hostname` requires `protocol: http/https`. |
| `internal/allocator/allocator.go` | Add port 15353 to reserved ports set. |
| `cmd/apply.go` | Replace worktree-based instance resolution with registry-based. Compute and store hostnames/protocols. Extend `buildTemplateVars` for `url` and `url:direct` fields. Update output to show hostnames. |
| `cmd/context.go` | Remove worktree dependency from `projectContext`. Replace with instance name resolved from registry. |
| `cmd/ports.go` | Show hostnames alongside ports when setup is active. |
| `cmd/status.go` | Show hostnames in global project list. Remove worktree-specific display logic. |
| `cmd/open.go` | Use `.test` URLs when setup is active. Fix "Run 'outport up' first" error message. |
| `cmd/unapply.go` | Handle new registry fields on removal. |
| `cmd/gc.go` | No functional changes needed — existing stale detection works with new fields. |
| `cmd/root.go` | Register new commands (setup, teardown, up, down, rename, promote). |
| `cmd/cmd_test.go` | Update existing tests for new instance model. Add tests for new commands. |
| `internal/ui/styles.go` | Add `HostnameStyle` for terminal output. |
| `go.mod` | Add `miekg/dns` and `fsnotify/fsnotify` dependencies. |

### Removed Files

| File | Reason |
|------|--------|
| `internal/worktree/worktree.go` | Replaced by registry-based instance resolution |
| `internal/worktree/worktree_test.go` | No longer needed |

---

## Chunk 1: Registry Extension

Extend the registry to store hostnames and protocols. Add lookup methods needed by instance resolution.

### Task 1: Add Hostnames and Protocols to Allocation struct

**Files:**
- Modify: `internal/registry/registry.go:10-13`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: Write test for new Allocation fields**

Add a test that creates an Allocation with hostnames and protocols, saves the registry, reloads it, and verifies the fields persist.

```go
func TestAllocationWithHostnames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg := &Registry{Projects: make(map[string]Allocation), path: path}
	reg.Set("myapp", "main", Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"rails": 24920},
		Hostnames:  map[string]string{"rails": "myapp.test"},
		Protocols:  map[string]string{"rails": "http"},
	})

	err := reg.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	alloc, ok := loaded.Get("myapp", "main")
	if !ok {
		t.Fatal("expected allocation")
	}
	if alloc.Hostnames["rails"] != "myapp.test" {
		t.Errorf("hostname: got %q, want %q", alloc.Hostnames["rails"], "myapp.test")
	}
	if alloc.Protocols["rails"] != "http" {
		t.Errorf("protocol: got %q, want %q", alloc.Protocols["rails"], "http")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/ -run TestAllocationWithHostnames -v`
Expected: FAIL — `Hostnames` and `Protocols` fields don't exist on Allocation.

- [ ] **Step 3: Extend Allocation struct**

In `internal/registry/registry.go`, add the new fields:

```go
type Allocation struct {
	ProjectDir string            `json:"project_dir"`
	Ports      map[string]int    `json:"ports"`
	Hostnames  map[string]string `json:"hostnames,omitempty"`
	Protocols  map[string]string `json:"protocols,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/registry/ -run TestAllocationWithHostnames -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat: add hostnames and protocols to registry Allocation"
```

### Task 2: Add FindByDir and FindByProject registry methods

**Files:**
- Modify: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: Write tests for FindByDir and FindByProject**

```go
func TestFindByDir(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("myapp", "main", Allocation{ProjectDir: "/src/myapp"})
	reg.Set("myapp", "bkrm", Allocation{ProjectDir: "/tmp/myapp-clone"})

	key, alloc, ok := reg.FindByDir("/src/myapp")
	if !ok {
		t.Fatal("expected to find by dir")
	}
	if key != "myapp/main" {
		t.Errorf("key: got %q, want %q", key, "myapp/main")
	}
	if alloc.ProjectDir != "/src/myapp" {
		t.Errorf("dir: got %q", alloc.ProjectDir)
	}

	_, _, ok = reg.FindByDir("/nonexistent")
	if ok {
		t.Error("expected not found for nonexistent dir")
	}
}

func TestFindByProject(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("myapp", "main", Allocation{ProjectDir: "/src/myapp"})
	reg.Set("myapp", "bkrm", Allocation{ProjectDir: "/tmp/myapp-clone"})
	reg.Set("other", "main", Allocation{ProjectDir: "/src/other"})

	instances := reg.FindByProject("myapp")
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	instances = reg.FindByProject("nonexistent")
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/registry/ -run "TestFindByDir|TestFindByProject" -v`
Expected: FAIL — methods don't exist.

- [ ] **Step 3: Implement FindByDir and FindByProject**

```go
// FindByDir searches for an allocation whose ProjectDir matches the given directory.
// Returns the registry key, allocation, and whether it was found.
func (r *Registry) FindByDir(dir string) (string, Allocation, bool) {
	for key, alloc := range r.Projects {
		if alloc.ProjectDir == dir {
			return key, alloc, true
		}
	}
	return "", Allocation{}, false
}

// FindByProject returns all registry keys that belong to the given project name.
// Keys are in "project/instance" format; this matches on the project prefix.
func (r *Registry) FindByProject(project string) map[string]Allocation {
	prefix := project + "/"
	result := make(map[string]Allocation)
	for key, alloc := range r.Projects {
		if strings.HasPrefix(key, prefix) {
			result[key] = alloc
		}
	}
	return result
}
```

Add `"strings"` to the imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/registry/ -run "TestFindByDir|TestFindByProject" -v`
Expected: PASS

- [ ] **Step 5: Run all registry tests**

Run: `go test ./internal/registry/ -v`
Expected: All PASS — no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat: add FindByDir and FindByProject to registry"
```

### Task 3: Add reserved port for DNS server in allocator

**Files:**
- Modify: `internal/allocator/allocator.go`
- Test: `internal/allocator/allocator_test.go`

- [ ] **Step 1: Write test that port 15353 is never allocated**

```go
func TestReservedPortSkipped(t *testing.T) {
	// Force hash to land on 15353 by using it as preferred, then mark it used
	// Instead, directly test that 15353 is treated as used
	usedPorts := map[int]bool{}
	// Allocate many ports and verify none is 15353
	for i := 0; i < 1000; i++ {
		port, err := Allocate("proj", "inst", fmt.Sprintf("svc%d", i), 0, usedPorts)
		if err != nil {
			t.Fatalf("Allocate svc%d: %v", i, err)
		}
		if port == ReservedDNSPort {
			t.Fatalf("allocated reserved DNS port %d for svc%d", ReservedDNSPort, i)
		}
		usedPorts[port] = true
	}
}

func TestReservedPortPreferredFallsBack(t *testing.T) {
	port, err := Allocate("proj", "inst", "svc", ReservedDNSPort, map[int]bool{})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if port == ReservedDNSPort {
		t.Fatalf("should not allocate reserved port even when preferred")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/allocator/ -run "TestReservedPort" -v`
Expected: FAIL — `ReservedDNSPort` not defined; 15353 may be allocated.

- [ ] **Step 3: Add reserved port constant and exclusion logic**

In `internal/allocator/allocator.go`:

```go
const (
	MinPort        = 10000
	MaxPort        = 39999
	portRange      = MaxPort - MinPort + 1
	ReservedDNSPort = 15353
)

var reservedPorts = map[int]bool{
	ReservedDNSPort: true,
}

func Allocate(project, instance, service string, preferred int, usedPorts map[int]bool) (int, error) {
	if preferred > 0 && !usedPorts[preferred] && !reservedPorts[preferred] {
		return preferred, nil
	}

	start := HashPort(project, instance, service)
	port := start
	for usedPorts[port] || reservedPorts[port] {
		port++
		if port > MaxPort {
			port = MinPort
		}
		if port == start {
			return 0, fmt.Errorf("no available ports in range %d-%d", MinPort, MaxPort)
		}
	}
	return port, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/allocator/ -run "TestReservedPort" -v`
Expected: PASS

- [ ] **Step 5: Run all allocator tests**

Run: `go test ./internal/allocator/ -v`
Expected: All PASS — no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/allocator/allocator.go internal/allocator/allocator_test.go
git commit -m "feat: reserve port 15353 for DNS server in allocator"
```

---

## Chunk 2: Instance Resolution

Replace worktree detection with registry-based instance resolution.

### Task 4: Create instance package with code generation

**Files:**
- Create: `internal/instance/instance.go`
- Create: `internal/instance/instance_test.go`

- [ ] **Step 1: Write tests for GenerateCode**

```go
package instance

import "testing"

func TestGenerateCode(t *testing.T) {
	used := map[string]bool{}
	code := GenerateCode(used)
	if len(code) != 4 {
		t.Fatalf("code length: got %d, want 4", len(code))
	}
	for _, c := range code {
		if !isConsonant(byte(c)) {
			t.Errorf("code %q contains non-consonant %c", code, c)
		}
	}
}

func TestGenerateCodeAvoidsCollisions(t *testing.T) {
	used := map[string]bool{}
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := GenerateCode(used)
		if codes[code] {
			t.Fatalf("duplicate code %q on iteration %d", code, i)
		}
		codes[code] = true
		used[code] = true
	}
}

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"main", false},
		{"feature-xyz", false},
		{"bkrm", false},
		{"agent-1", false},
		{"", true},
		{"UPPER", true},
		{"has space", true},
		{"has_underscore", true},
	}
	for _, tt := range tests {
		err := ValidateName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateName(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/instance/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement instance package**

```go
package instance

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
)

// Consonant alphabet: no vowels (prevents offensive words), no ambiguous 'l'.
const consonants = "bcdfghjkmnpqrstvwxz"

var validNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// GenerateCode produces a random 4-character consonant code that is not in the used set.
func GenerateCode(used map[string]bool) string {
	for {
		code := randomCode(4)
		if !used[code] {
			return code
		}
	}
}

func randomCode(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(consonants))))
		b[i] = consonants[n.Int64()]
	}
	return string(b)
}

func isConsonant(c byte) bool {
	for i := 0; i < len(consonants); i++ {
		if consonants[i] == c {
			return true
		}
	}
	return false
}

// ValidateName checks that an instance name is valid: lowercase alphanumeric and hyphens.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("instance name %q must be lowercase alphanumeric and hyphens only", name)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/instance/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/instance/
git commit -m "feat: add instance package with code generation and validation"
```

### Task 5: Add instance resolution logic

**Files:**
- Modify: `internal/instance/instance.go`
- Modify: `internal/instance/instance_test.go`

- [ ] **Step 1: Write tests for Resolve**

```go
func TestResolveExistingInstance(t *testing.T) {
	projects := map[string]struct{ dir string }{
		"myapp/main": {dir: "/src/myapp"},
		"myapp/bkrm": {dir: "/tmp/myapp-clone"},
	}
	reg := buildTestRegistry(projects)

	name, isNew, err := Resolve(reg, "myapp", "/src/myapp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "main" {
		t.Errorf("name: got %q, want %q", name, "main")
	}
	if isNew {
		t.Error("expected isNew=false for existing instance")
	}
}

func TestResolveFirstInstance(t *testing.T) {
	reg := buildTestRegistry(nil)

	name, isNew, err := Resolve(reg, "myapp", "/src/myapp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "main" {
		t.Errorf("name: got %q, want %q", name, "main")
	}
	if !isNew {
		t.Error("expected isNew=true for first instance")
	}
}

func TestResolveSubsequentInstance(t *testing.T) {
	projects := map[string]struct{ dir string }{
		"myapp/main": {dir: "/src/myapp"},
	}
	reg := buildTestRegistry(projects)

	name, isNew, err := Resolve(reg, "myapp", "/tmp/myapp-clone")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name == "main" {
		t.Error("subsequent instance should not be 'main'")
	}
	if len(name) != 4 {
		t.Errorf("expected 4-char code, got %q", name)
	}
	if !isNew {
		t.Error("expected isNew=true for new instance")
	}
}
```

Add a helper at the top of the test file:

```go
import (
	"strings"
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

func buildTestRegistry(projects map[string]struct{ dir string }) *registry.Registry {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	for key, p := range projects {
		parts := strings.SplitN(key, "/", 2)
		reg.Set(parts[0], parts[1], registry.Allocation{ProjectDir: p.dir})
	}
	return reg
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/instance/ -run "TestResolve" -v`
Expected: FAIL — `Resolve` function doesn't exist.

- [ ] **Step 3: Implement Resolve**

```go
// Resolve determines the instance name for a project in a given directory.
// Returns the instance name, whether it's newly created, and any error.
// NOTE: Resolve does NOT modify the registry. The caller is responsible for
// calling reg.Set() and reg.Save() to persist the new instance.
func Resolve(reg *registry.Registry, project, dir string) (string, bool, error) {
	// Step 1: Check if this directory is already registered
	key, _, ok := reg.FindByDir(dir)
	if ok {
		// Extract instance from "project/instance" key
		parts := strings.SplitN(key, "/", 2)
		return parts[1], false, nil
	}

	// Step 2: Check if any instance of this project exists
	existing := reg.FindByProject(project)
	if len(existing) == 0 {
		return "main", true, nil
	}

	// Step 3: Generate a unique code
	usedNames := make(map[string]bool)
	for key := range existing {
		parts := strings.SplitN(key, "/", 2)
		usedNames[parts[1]] = true
	}
	code := GenerateCode(usedNames)
	return code, true, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/instance/ -run "TestResolve" -v`
Expected: PASS

- [ ] **Step 5: Run all instance tests**

Run: `go test ./internal/instance/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/instance/
git commit -m "feat: add instance resolution with auto-generated codes"
```

### Task 6: Remove worktree package (combined with Task 8)

**Deferred:** The worktree package deletion is combined with Task 8 (apply command rework) to avoid a broken build state between commits. Do NOT delete the worktree package until Task 8, where it is removed and all references are updated in a single atomic commit.

---

## Chunk 3: Config and Template Changes

Extend config validation and the template system to support hostnames and the `:modifier` syntax.

### Task 7: Add hostname validation and template modifier support to config

**Files:**
- Modify: `internal/config/config.go:14-20` (regex and validFields)
- Modify: `internal/config/config.go:260-316` (validate function)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write tests for hostname validation**

```go
func TestHostnameRequiresHTTPProtocol(t *testing.T) {
	yaml := `
name: myapp
services:
  postgres:
    env_var: PGPORT
    hostname: myapp
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for hostname without http protocol")
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error should mention hostname, got: %v", err)
	}
}

func TestHostnameValidCharacters(t *testing.T) {
	tests := []struct {
		hostname string
		wantErr  bool
	}{
		{"myapp", false},
		{"portal.myapp", false},
		{"myapp-web", false},
		{"my_app", true},   // underscores invalid in DNS
		{"MY_APP", true},   // uppercase invalid
		{"my app", true},   // spaces invalid
		{"othername", true}, // must contain project name "myapp"
	}
	for _, tt := range tests {
		yaml := fmt.Sprintf(`
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: %s
`, tt.hostname)
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
		_, err := Load(dir)
		if (err != nil) != tt.wantErr {
			t.Errorf("hostname %q: err=%v, wantErr=%v", tt.hostname, err, tt.wantErr)
		}
	}
}

func TestHostnameMustContainProjectName(t *testing.T) {
	yaml := `
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: othername
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error: hostname must contain project name")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestHostname" -v`
Expected: FAIL — validation doesn't exist yet.

- [ ] **Step 3: Add hostname validation to config**

In `internal/config/config.go`, add a hostname validation regex and extend the `validate()` function:

```go
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)
```

Add to `validate()`:

```go
// Validate hostname fields
for name, svc := range c.Services {
	if svc.Hostname != "" {
		if svc.Protocol != "http" && svc.Protocol != "https" {
			return fmt.Errorf("service %q: hostname requires protocol http or https", name)
		}
		if !hostnameRe.MatchString(svc.Hostname) {
			return fmt.Errorf("service %q: hostname %q contains invalid characters (use lowercase alphanumeric, hyphens, dots)", name, svc.Hostname)
		}
		if !strings.Contains(svc.Hostname, c.Name) {
			return fmt.Errorf("service %q: hostname %q must contain project name %q", name, svc.Hostname, c.Name)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "TestHostname" -v`
Expected: PASS

- [ ] **Step 5: Write tests for template modifier parsing**

```go
func TestTemplateModifierParsing(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    protocol: http
    hostname: myapp
derived:
  API_URL:
    value: "${rails.url:direct}/api"
    env_file: .env
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	vars := map[string]string{
		"rails.port":       "24920",
		"rails.hostname":   "myapp.test",
		"rails.url":        "http://myapp.test",
		"rails.url:direct": "http://localhost:24920",
	}
	resolved := ResolveDerived(cfg.Derived, vars)
	val := resolved["API_URL"][".env"]
	if val != "http://localhost:24920/api" {
		t.Errorf("got %q, want %q", val, "http://localhost:24920/api")
	}
}

func TestTemplateModifierValidation(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    protocol: http
    hostname: myapp
derived:
  BAD:
    value: "${rails.url:bogus}"
    env_file: .env
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unrecognized modifier")
	}
}

func TestURLFieldValidation(t *testing.T) {
	yaml := `
name: myapp
services:
  rails:
    env_var: PORT
    protocol: http
    hostname: myapp
derived:
  SITE_URL:
    value: "${rails.url}"
    env_file: .env
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(yaml), 0644)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected no error for url field, got: %v", err)
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestTemplate" -v`
Expected: FAIL — `url` not in validFields, modifier regex doesn't match.

- [ ] **Step 7: Extend template regex and validation**

Update the regex and validFields in `config.go`:

```go
// Matches ${service.field} and ${service.field:modifier}
var templateVarRe = regexp.MustCompile(`\$\{(\w+)\.(\w+)(?::(\w+))?\}`)

var validFields = map[string]bool{
	"port":     true,
	"hostname": true,
	"url":      true,
}

var validModifiers = map[string]map[string]bool{
	"url": {"direct": true},
}
```

Update `validateTemplateRefs` to handle the modifier:

```go
func validateTemplateRefs(derivedName, template string, services map[string]Service) error {
	matches := templateVarRe.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		svcName := m[1]
		field := m[2]
		modifier := ""
		if len(m) > 3 {
			modifier = m[3]
		}

		if _, ok := services[svcName]; !ok {
			return fmt.Errorf("derived %q: references unknown service %q", derivedName, svcName)
		}
		if !validFields[field] {
			return fmt.Errorf("derived %q: unknown field %q (valid: port, hostname, url)", derivedName, field)
		}
		if modifier != "" {
			mods, ok := validModifiers[field]
			if !ok || !mods[modifier] {
				return fmt.Errorf("derived %q: unknown modifier %q for field %q", derivedName, modifier, field)
			}
		}
	}
	return nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "TestTemplate|TestURL" -v`
Expected: PASS

- [ ] **Step 9: Run all config tests to check for regressions**

Run: `go test ./internal/config/ -v`
Expected: All PASS

- [ ] **Step 10: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add hostname validation and template modifier support"
```

---

## Chunk 4: Apply Command Rework

Rework the apply command to use registry-based instance resolution, compute hostnames, and support new template fields.

### Task 8: Update projectContext and apply workflow

**Files:**
- Modify: `cmd/context.go`
- Modify: `cmd/apply.go`
- Delete: `internal/worktree/worktree.go` (combined from Task 6)
- Delete: `internal/worktree/worktree_test.go` (combined from Task 6)
- Modify: `cmd/root.go` (if needed to remove worktree import)
- Test: `cmd/cmd_test.go`

- [ ] **Step 0: Delete worktree package**

```bash
rm -rf internal/worktree/
```

This is the deferred deletion from Task 6, done atomically with the apply rework so the build never breaks.

- [ ] **Step 1: Update projectContext to remove worktree dependency**

Replace `cmd/context.go` content:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/instance"
	"github.com/outport-app/outport/internal/registry"
)

type projectContext struct {
	Dir      string
	Cfg      *config.Config
	Instance string
	IsNew    bool
	Reg      *registry.Registry
}

func loadProjectContext() (*projectContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	dir, err := config.FindDir(cwd)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	inst, isNew, err := instance.Resolve(reg, cfg.Name, dir)
	if err != nil {
		return nil, err
	}

	return &projectContext{
		Dir:      dir,
		Cfg:      cfg,
		Instance: inst,
		IsNew:    isNew,
		Reg:      reg,
	}, nil
}

func loadRegistry() (*registry.Registry, error) {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	return registry.Load(regPath)
}
```

- [ ] **Step 2: Add hostname computation functions to apply.go**

Add these functions to `cmd/apply.go`:

```go
// computeHostnames builds hostname map for an allocation.
// For "main" instance, hostnames are stem + ".test".
// For other instances, the project name in the stem is suffixed with "-instance".
func computeHostnames(cfg *config.Config, instanceName string) map[string]string {
	hostnames := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc.Hostname == "" {
			continue
		}
		stem := svc.Hostname
		if instanceName != "main" {
			// Replace rightmost occurrence of project name with project-instance
			idx := strings.LastIndex(stem, cfg.Name)
			if idx >= 0 {
				stem = stem[:idx] + cfg.Name + "-" + instanceName + stem[idx+len(cfg.Name):]
			}
		}
		hostnames[name] = stem + ".test"
	}
	return hostnames
}

// computeProtocols builds protocol map from config (only for services with protocol set).
func computeProtocols(cfg *config.Config) map[string]string {
	protocols := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc.Protocol != "" {
			protocols[name] = svc.Protocol
		}
	}
	return protocols
}
```

- [ ] **Step 3: Update buildTemplateVars to include url and url:direct**

```go
func buildTemplateVars(cfg *config.Config, ports map[string]int, hostnames map[string]string) map[string]string {
	vars := make(map[string]string)
	for name, svc := range cfg.Services {
		portStr := fmt.Sprintf("%d", ports[name])
		vars[name+".port"] = portStr

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h
			protocol := svc.Protocol
			if protocol == "" {
				protocol = "http"
			}
			vars[name+".url"] = fmt.Sprintf("%s://%s", protocol, h)
			vars[name+".url:direct"] = fmt.Sprintf("http://localhost:%s", portStr)
		} else {
			hostname := svc.Hostname
			if hostname == "" {
				hostname = "localhost"
			}
			vars[name+".hostname"] = hostname
		}
	}
	return vars
}
```

- [ ] **Step 4: Update runApply to use new instance model**

Update the `runApply` function to:
- Use `ctx.Instance` instead of `ctx.WT.Instance`
- Call `computeHostnames()` and `computeProtocols()`
- Store hostnames and protocols in the allocation
- Pass hostnames to `buildTemplateVars()`
- Check hostname uniqueness across registry
- Print instance registration message if new
- Print setup hint if not configured

This is a significant rewrite of `runApply`. The key changes:
- Replace `ctx.WT.Instance` with `ctx.Instance` everywhere
- After port allocation, compute hostnames and protocols
- Before saving to registry, validate hostname uniqueness
- Store the enriched allocation (with hostnames/protocols)
- Update `buildTemplateVars` call to pass hostnames
- Add setup detection and hint output

- [ ] **Step 5: Update existing apply tests**

Update tests in `cmd/cmd_test.go` that reference worktree:
- Remove `.git` directory creation from `setupProject` (it's no longer needed for worktree detection)
- Update assertions that check for instance names
- Add new tests for multi-instance scenarios and hostname computation

- [ ] **Step 6: Run all tests**

Run: `go test ./... -v`
Expected: All PASS (or identify what still needs updating).

- [ ] **Step 7: Commit**

```bash
rm -rf internal/worktree/
git add -A internal/worktree/ cmd/context.go cmd/apply.go cmd/cmd_test.go cmd/root.go
git commit -m "feat: rework apply with registry-based instance resolution and hostnames"
```

### Task 9: Update remaining CLI commands for new instance model

**Files:**
- Modify: `cmd/status.go`
- Modify: `cmd/ports.go`
- Modify: `cmd/open.go`
- Modify: `cmd/unapply.go`
- Modify: `cmd/gc.go`
- Modify: `internal/ui/styles.go`
- Test: `cmd/cmd_test.go`

- [ ] **Step 1: Add HostnameStyle to ui package**

In `internal/ui/styles.go`:

```go
var HostnameStyle = lipgloss.NewStyle().Foreground(Cyan)
```

- [ ] **Step 2: Update status.go**

- Replace worktree-specific display logic with instance display
- Show hostnames in project listing when available
- Update `formatProjectKey` to not mention "worktree"
- Replace `currentProjectKey` to use registry lookup instead of worktree detection

- [ ] **Step 3: Update ports.go**

- Show hostnames alongside ports (e.g., `PORT  24920  →  unio.test`)
- Read hostnames from allocation in registry

- [ ] **Step 4: Update open.go**

- Fix error message: change "Run 'outport up' first" to "Run 'outport apply' first"
- When setup is active (check for resolver file), use `.test` URL instead of `localhost:PORT`
- Add platform setup detection helper

- [ ] **Step 5: Update unapply.go**

- Use `ctx.Instance` instead of `ctx.WT.Instance`
- Existing cleanup logic works since it removes the entire registry entry

- [ ] **Step 6: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

- [ ] **Step 7: Run linter**

Run: `just lint`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add cmd/ internal/ui/styles.go
git commit -m "feat: update CLI commands for instance model and hostname display"
```

---

## Chunk 5: Daemon — DNS Server

Build the embedded DNS server.

### Task 10: Implement DNS server

**Files:**
- Create: `internal/daemon/dns.go`
- Create: `internal/daemon/dns_test.go`

- [ ] **Step 1: Add miekg/dns dependency**

Run: `go get github.com/miekg/dns`

- [ ] **Step 2: Write DNS server tests**

```go
package daemon

import (
	"testing"

	"github.com/miekg/dns"
)

func TestDNSServerResolvesTestDomain(t *testing.T) {
	srv, addr := startTestDNS(t)
	defer srv.Shutdown()

	// Query for foo.test
	m := new(dns.Msg)
	m.SetQuestion("foo.test.", dns.TypeA)
	r, err := dns.Exchange(m, addr)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(r.Answer))
	}
	a, ok := r.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("expected A record")
	}
	if a.A.String() != "127.0.0.1" {
		t.Errorf("got %s, want 127.0.0.1", a.A.String())
	}
}

func TestDNSServerResolvesSubdomain(t *testing.T) {
	srv, addr := startTestDNS(t)
	defer srv.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("portal.unio.test.", dns.TypeA)
	r, err := dns.Exchange(m, addr)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if len(r.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(r.Answer))
	}
}

func TestDNSServerRejectsNonTestDomain(t *testing.T) {
	srv, addr := startTestDNS(t)
	defer srv.Shutdown()

	m := new(dns.Msg)
	m.SetQuestion("foo.com.", dns.TypeA)
	r, err := dns.Exchange(m, addr)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if r.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got %d", r.Rcode)
	}
}
```

Add helper:

```go
func startTestDNS(t *testing.T) (*dns.Server, string) {
	t.Helper()
	srv, err := NewDNSServer("127.0.0.1:0") // random port
	if err != nil {
		t.Fatalf("NewDNSServer: %v", err)
	}
	go srv.ActivateAndServe()
	// Wait for server to be ready
	addr := srv.PacketConn.LocalAddr().String()
	return srv, addr
}
```

Note: The exact test helper may need adjustment based on `miekg/dns` server API (binding to port 0 for random port). Consult `miekg/dns` docs during implementation for the correct approach to start a test server.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run "TestDNS" -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 4: Implement DNS server**

```go
package daemon

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

const dnsTTL = 60 // seconds

// NewDNSServer creates a DNS server that resolves *.test to 127.0.0.1.
func NewDNSServer(addr string) (*dns.Server, error) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		for _, q := range r.Question {
			if q.Qtype == dns.TypeA && strings.HasSuffix(q.Name, ".test.") {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    dnsTTL,
					},
					A: net.ParseIP("127.0.0.1"),
				})
			}
		}

		if len(m.Answer) == 0 {
			m.Rcode = dns.RcodeNameError
		}

		w.WriteMsg(m)
	})

	srv := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: handler,
	}
	return srv, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run "TestDNS" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/daemon/dns.go internal/daemon/dns_test.go go.mod go.sum
git commit -m "feat: add embedded DNS server for .test domain resolution"
```

---

## Chunk 6: Daemon — Route Table and HTTP Proxy

Build the route table manager and HTTP reverse proxy with WebSocket support.

### Task 11: Implement route table builder

**Files:**
- Create: `internal/daemon/routes.go`
- Create: `internal/daemon/routes_test.go`

- [ ] **Step 1: Write tests for route table building from registry**

```go
package daemon

import (
	"path/filepath"
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

func TestBuildRoutes(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"rails": 24920, "postgres": 5432},
		Hostnames:  map[string]string{"rails": "myapp.test"},
		Protocols:  map[string]string{"rails": "http"},
	})
	reg.Set("other", "main", registry.Allocation{
		ProjectDir: "/src/other",
		Ports:      map[string]int{"web": 31000},
		Hostnames:  map[string]string{"web": "other.test"},
		Protocols:  map[string]string{"web": "http"},
	})

	routes := BuildRoutes(reg)
	if routes["myapp.test"] != 24920 {
		t.Errorf("myapp.test: got %d, want 24920", routes["myapp.test"])
	}
	if routes["other.test"] != 31000 {
		t.Errorf("other.test: got %d, want 31000", routes["other.test"])
	}
	if _, ok := routes["postgres"]; ok {
		t.Error("postgres should not have a route (no hostname)")
	}
}

func TestBuildRoutesSkipsNilHostnames(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("old", "main", registry.Allocation{
		ProjectDir: "/src/old",
		Ports:      map[string]int{"web": 12000},
		// No Hostnames or Protocols (old format)
	})

	routes := BuildRoutes(reg)
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for old-format entry, got %d", len(routes))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run "TestBuildRoutes" -v`
Expected: FAIL — `BuildRoutes` doesn't exist.

- [ ] **Step 3: Implement BuildRoutes**

```go
package daemon

import (
	"github.com/outport-app/outport/internal/registry"
)

// BuildRoutes constructs a hostname → port routing table from the registry.
// Only entries with hostnames and HTTP/HTTPS protocols are included.
func BuildRoutes(reg *registry.Registry) map[string]int {
	routes := make(map[string]int)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			proto := alloc.Protocols[svcName]
			if proto == "http" || proto == "https" {
				routes[hostname] = alloc.Ports[svcName]
			}
		}
	}
	return routes
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run "TestBuildRoutes" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/routes.go internal/daemon/routes_test.go
git commit -m "feat: add route table builder from registry"
```

### Task 12: Implement HTTP reverse proxy

**Files:**
- Create: `internal/daemon/proxy.go`
- Create: `internal/daemon/proxy_test.go`

- [ ] **Step 1: Write tests for proxy routing**

```go
package daemon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRoutesToBackend(t *testing.T) {
	// Start a backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	// Extract port from backend URL
	port := backendPort(t, backend)

	routes := &RouteTable{}
	routes.Update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from backend" {
		t.Errorf("got %q, want %q", body, "hello from backend")
	}
}

func TestProxyUnknownHostReturnsError(t *testing.T) {
	routes := &RouteTable{}
	routes.Update(map[string]int{})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "unknown.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}

func TestProxyBackendDownReturnsError(t *testing.T) {
	routes := &RouteTable{}
	routes.Update(map[string]int{"myapp.test": 59999}) // nothing on this port

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "myapp.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run "TestProxy" -v`
Expected: FAIL — `RouteTable`, `NewProxy` don't exist.

- [ ] **Step 3: Implement RouteTable**

```go
package daemon

import "sync"

// RouteTable is a thread-safe hostname → port mapping.
type RouteTable struct {
	mu     sync.RWMutex
	routes map[string]int
}

// Lookup returns the port for a hostname, or 0 if not found.
func (rt *RouteTable) Lookup(hostname string) (int, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	port, ok := rt.routes[hostname]
	return port, ok
}

// Update swaps the routing table atomically.
func (rt *RouteTable) Update(routes map[string]int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.routes = routes
}
```

- [ ] **Step 4: Implement NewProxy**

```go
package daemon

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewProxy creates an HTTP reverse proxy that routes by Host header.
func NewProxy(routes *RouteTable) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname := r.Host
		// Strip port if present (e.g., "myapp.test:80" → "myapp.test")
		if idx := strings.LastIndex(hostname, ":"); idx != -1 {
			hostname = hostname[:idx]
		}

		port, ok := routes.Lookup(hostname)
		if !ok {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "No project is configured for %s.\nRun `outport apply` with a matching hostname.\n", hostname)
			return
		}

		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "%s is not running.\nStart your app and try again.\n", hostname)
		}
		proxy.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Add test helper**

```go
import (
	"net/url"
	"strconv"
)

func backendPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return port
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run "TestProxy" -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/daemon/proxy.go internal/daemon/proxy_test.go internal/daemon/routes.go
git commit -m "feat: add HTTP reverse proxy with route table"
```

### Task 13: Add WebSocket upgrade support to proxy

**Files:**
- Modify: `internal/daemon/proxy.go`
- Modify: `internal/daemon/proxy_test.go`

- [ ] **Step 1: Write WebSocket upgrade test**

```go
func TestProxyWebSocketUpgrade(t *testing.T) {
	// Start a backend that accepts WebSocket upgrades
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Connection") != "Upgrade" || r.Header.Get("Upgrade") != "websocket" {
			t.Error("backend did not receive upgrade headers")
			w.WriteHeader(400)
			return
		}
		// Hijack and echo
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("backend doesn't support hijacking")
		}
		conn, buf, _ := hj.Hijack()
		defer conn.Close()
		buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		buf.WriteString("ws-hello")
		buf.Flush()
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.Update(map[string]int{"myapp.test": port})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/cable", nil)
	req.Host = "myapp.test"
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}
}
```

Note: WebSocket testing at the HTTP level may need adjustment based on how `httputil.ReverseProxy` handles upgrades. The standard library proxy does handle WebSocket upgrades transparently in recent Go versions (1.20+). Verify during implementation and adjust the test approach if needed — a full gorilla/websocket test client may be more reliable.

- [ ] **Step 2: Run test to verify behavior**

Run: `go test ./internal/daemon/ -run "TestProxyWebSocket" -v`

If the standard `httputil.ReverseProxy` already handles WebSocket upgrades (likely in Go 1.26), this test should pass without changes. If not, implement the hijack-and-relay approach described in the spec.

- [ ] **Step 3: If needed, add explicit WebSocket handling**

Only if the standard proxy doesn't handle upgrades: add a check in the handler before delegating to `httputil.ReverseProxy`:

```go
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "Upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
```

If WebSocket upgrade is detected, hijack both sides and relay with `io.Copy` in both directions.

- [ ] **Step 4: Run all daemon tests**

Run: `go test ./internal/daemon/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/
git commit -m "feat: add WebSocket upgrade support to reverse proxy"
```

---

## Chunk 7: Daemon — Registry Watcher and Entry Point

Wire up fsnotify for registry watching and create the daemon entry point.

### Task 14: Implement registry file watcher

**Files:**
- Modify: `internal/daemon/routes.go`
- Test: `internal/daemon/routes_test.go`

- [ ] **Step 1: Add fsnotify dependency**

Run: `go get github.com/fsnotify/fsnotify`

- [ ] **Step 2: Write test for registry watcher triggering route rebuild**

```go
func TestRouteWatcherRebuildsOnChange(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Write initial registry
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 24920},
		Hostnames:  map[string]string{"web": "myapp.test"},
		Protocols:  map[string]string{"web": "http"},
	})
	// Save using registry's own Save method requires setting path —
	// for test, write JSON directly
	writeTestRegistry(t, regPath, reg)

	rt := &RouteTable{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- WatchAndRebuild(ctx, regPath, rt)
	}()

	// Wait for initial load
	time.Sleep(100 * time.Millisecond)
	port, ok := rt.Lookup("myapp.test")
	if !ok || port != 24920 {
		t.Fatalf("initial route: ok=%v port=%d", ok, port)
	}

	// Update registry
	reg.Set("other", "main", registry.Allocation{
		ProjectDir: "/src/other",
		Ports:      map[string]int{"web": 31000},
		Hostnames:  map[string]string{"web": "other.test"},
		Protocols:  map[string]string{"web": "http"},
	})
	writeTestRegistry(t, regPath, reg)

	// Wait for watcher to pick up change
	time.Sleep(500 * time.Millisecond)
	port, ok = rt.Lookup("other.test")
	if !ok || port != 31000 {
		t.Fatalf("updated route: ok=%v port=%d", ok, port)
	}

	cancel()
}
```

- [ ] **Step 3: Implement WatchAndRebuild**

```go
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/outport-app/outport/internal/registry"
)

// WatchAndRebuild watches the registry file and rebuilds routes on changes.
// It watches the parent directory (not the file) to handle atomic writes correctly.
func WatchAndRebuild(ctx context.Context, regPath string, rt *RouteTable) error {
	// Initial load
	if err := rebuildFromFile(regPath, rt); err != nil {
		return fmt.Errorf("initial route build: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	dir := filepath.Dir(regPath)
	base := filepath.Base(regPath)
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch directory %s: %w", dir, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Base(event.Name) == base &&
				(event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
				rebuildFromFile(regPath, rt) // best-effort; log errors in production
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

func rebuildFromFile(regPath string, rt *RouteTable) error {
	data, err := os.ReadFile(regPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Registry deleted — keep existing routes (spec: retain last-known table)
			return nil
		}
		return err
	}
	var reg registry.Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return err
	}
	routes := BuildRoutes(&reg)
	rt.Update(routes)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run "TestRouteWatcher" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/routes.go internal/daemon/routes_test.go go.mod go.sum
git commit -m "feat: add fsnotify registry watcher for route table rebuilds"
```

### Task 15: Implement daemon entry point

**Files:**
- Create: `internal/daemon/daemon.go`
- Create: `internal/daemon/daemon_test.go`

- [ ] **Step 1: Write test for daemon startup and shutdown**

```go
func TestDaemonStartsAndStops(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")
	writeTestRegistry(t, regPath, &registry.Registry{
		Projects: make(map[string]registry.Allocation),
	})

	cfg := &DaemonConfig{
		DNSAddr:      "127.0.0.1:0",
		ProxyAddr:    "127.0.0.1:0",
		RegistryPath: regPath,
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	err = <-errCh
	if err != nil && err != context.Canceled {
		t.Fatalf("Run: %v", err)
	}
}
```

- [ ] **Step 2: Implement Daemon**

```go
package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/miekg/dns"
)

// DaemonConfig holds configuration for the daemon process.
type DaemonConfig struct {
	DNSAddr      string // UDP address for DNS server (e.g., "127.0.0.1:15353")
	ProxyAddr    string // TCP address for HTTP proxy (e.g., ":80")
	RegistryPath string // Path to registry.json
	Listener     net.Listener // Optional: pre-bound listener (for launchd socket activation)
}

// Daemon coordinates the DNS server, HTTP proxy, and route watcher.
type Daemon struct {
	cfg    *DaemonConfig
	routes *RouteTable
	dns    *dns.Server
	proxy  *http.Server
}

// New creates a new Daemon instance.
func New(cfg *DaemonConfig) (*Daemon, error) {
	routes := &RouteTable{}

	dnsSrv, err := NewDNSServer(cfg.DNSAddr)
	if err != nil {
		return nil, fmt.Errorf("create DNS server: %w", err)
	}

	proxyHandler := NewProxy(routes)
	httpSrv := &http.Server{
		Addr:    cfg.ProxyAddr,
		Handler: proxyHandler,
	}

	return &Daemon{
		cfg:    cfg,
		routes: routes,
		dns:    dnsSrv,
		proxy:  httpSrv,
	}, nil
}

// Run starts the daemon and blocks until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	errCh := make(chan error, 3)

	// Start route watcher
	go func() {
		errCh <- WatchAndRebuild(ctx, d.cfg.RegistryPath, d.routes)
	}()

	// Start DNS server
	go func() {
		errCh <- d.dns.ListenAndServe()
	}()

	// Start HTTP proxy
	go func() {
		var err error
		if d.cfg.Listener != nil {
			err = d.proxy.Serve(d.cfg.Listener)
		} else {
			err = d.proxy.ListenAndServe()
		}
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		d.dns.Shutdown()
		d.proxy.Close()
		return nil
	case err := <-errCh:
		d.dns.Shutdown()
		d.proxy.Close()
		return err
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/daemon/ -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/daemon/daemon.go internal/daemon/daemon_test.go
git commit -m "feat: add daemon entry point coordinating DNS, proxy, and watcher"
```

---

## Chunk 8: Platform Layer and CLI Commands

macOS platform implementation and the new CLI commands (setup, teardown, up, down, rename, promote).

### Task 16: Implement macOS platform layer

**Files:**
- Create: `internal/platform/platform.go`
- Create: `internal/platform/darwin.go`
- Create: `internal/platform/darwin_test.go`

- [ ] **Step 1: Define platform interface**

```go
package platform

// SetupResult contains information about the setup operation.
type SetupResult struct {
	ResolverPath string
	PlistPath    string
}

// IsSetup checks if the DNS/proxy infrastructure is configured.
func IsSetup() bool {
	return isResolverInstalled() && isPlistInstalled()
}
```

- [ ] **Step 2: Implement macOS-specific functions**

In `darwin.go` (build-tagged for macOS):

```go
//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	resolverDir  = "/etc/resolver"
	resolverFile = "/etc/resolver/test"
	plistName    = "dev.outport.daemon.plist"
	dnsPort      = 15353
)

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func isResolverInstalled() bool {
	_, err := os.Stat(resolverFile)
	return err == nil
}

func isPlistInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

// WriteResolverFile creates /etc/resolver/test via sudo.
func WriteResolverFile() error {
	content := fmt.Sprintf("nameserver 127.0.0.1\nport %d\n", dnsPort)
	cmd := exec.Command("sudo", "tee", resolverFile)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write resolver file: %w", err)
	}
	return nil
}

// RemoveResolverFile removes /etc/resolver/test via sudo.
func RemoveResolverFile() error {
	cmd := exec.Command("sudo", "rm", "-f", resolverFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WritePlist writes the LaunchAgent plist with socket activation for port 80.
func WritePlist(outportBinary string) error {
	plist := generatePlist(outportBinary)
	return os.WriteFile(plistPath(), []byte(plist), 0644)
}

// RemovePlist removes the LaunchAgent plist.
func RemovePlist() error {
	return os.Remove(plistPath())
}

// LoadAgent loads the LaunchAgent via launchctl.
func LoadAgent() error {
	return exec.Command("launchctl", "load", plistPath()).Run()
}

// UnloadAgent unloads the LaunchAgent via launchctl.
func UnloadAgent() error {
	return exec.Command("launchctl", "unload", plistPath()).Run()
}

func generatePlist(binary string) string {
	// Generate plist XML with socket activation for port 80
	// This will need the exact plist format for launchd socket activation
	// Modeled after puma-dev's approach
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>dev.outport.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>Sockets</key>
	<dict>
		<key>Socket</key>
		<dict>
			<key>SockNodeName</key>
			<string>127.0.0.1</string>
			<key>SockServiceName</key>
			<string>80</string>
		</dict>
	</dict>
	<key>StandardOutPath</key>
	<string>/tmp/outport-daemon.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/outport-daemon.log</string>
</dict>
</plist>`, binary)
}
```

- [ ] **Step 3: Write tests for what's testable without root**

```go
//go:build darwin

package platform

import "testing"

func TestPlistGeneration(t *testing.T) {
	plist := generatePlist("/usr/local/bin/outport")
	if !strings.Contains(plist, "dev.outport.daemon") {
		t.Error("plist missing label")
	}
	if !strings.Contains(plist, "/usr/local/bin/outport") {
		t.Error("plist missing binary path")
	}
	if !strings.Contains(plist, "SockServiceName") {
		t.Error("plist missing socket activation")
	}
}

func TestIsSetupReturnsFalseWhenNotConfigured(t *testing.T) {
	// On a clean test system, setup should be false
	// (unless the developer actually has outport set up)
	// This is more of a smoke test
	_ = IsSetup()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/platform/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/platform/
git commit -m "feat: add macOS platform layer for resolver and LaunchAgent management"
```

### Task 17: Add daemon command (hidden, invoked by launchd)

**Files:**
- Create: `cmd/daemon.go`

- [ ] **Step 1: Implement daemon command**

This is a hidden command that launchd invokes. It's not user-facing.

```go
package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/outport-app/outport/internal/daemon"
	"github.com/outport-app/outport/internal/registry"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the DNS and proxy daemon (invoked by launchd)",
	Hidden: true,
	RunE:   runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}

	cfg := &daemon.DaemonConfig{
		DNSAddr:      "127.0.0.1:15353",
		ProxyAddr:    ":80",
		RegistryPath: regPath,
		// TODO: Accept launchd socket via launch_activate_socket() for port 80
	}

	d, err := daemon.New(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		cancel()
	}()

	return d.Run(ctx)
}
```

**Important: launchd socket activation is required for port 80.** The daemon runs as the current user (not root), so it cannot bind port 80 directly. launchd (running as root) binds the socket and passes the file descriptor via `launch_activate_socket()`. During development and testing, use a high port (e.g., 18080) that doesn't require privileges, and pass it via the `--port` flag or `OUTPORT_PROXY_PORT` env var. The cgo implementation of `launch_activate_socket()` is required for production use. Study puma-dev's `cmd/puma-dev/main_darwin.go` for the cgo pattern.

- [ ] **Step 2: Commit**

```bash
git add cmd/daemon.go
git commit -m "feat: add hidden daemon command for DNS and proxy"
```

### Task 18: Add setup, teardown, up, down commands

**Files:**
- Create: `cmd/setup.go`
- Create: `cmd/updown.go`

- [ ] **Step 1: Implement setup command**

```go
package cmd

import (
	"fmt"
	"os/exec"

	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure local DNS and reverse proxy for .test domains",
	Long:  "One-time setup: creates /etc/resolver/test and installs a LaunchAgent for the Outport daemon. Requires sudo for the resolver file.",
	RunE:  runSetup,
}

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Remove local DNS and reverse proxy configuration",
	RunE:  runTeardown,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(teardownCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	if platform.IsSetup() {
		fmt.Println("Outport DNS and proxy are already configured.")
		return nil
	}

	// Check if port 80 is in use
	if err := checkPort80(); err != nil {
		return err
	}

	// Find outport binary path
	binary, err := exec.LookPath("outport")
	if err != nil {
		return fmt.Errorf("outport binary not found in PATH: %w", err)
	}

	fmt.Print("  Installing LaunchAgent ... ")
	if err := platform.WritePlist(binary); err != nil {
		return err
	}
	fmt.Println("done")

	fmt.Print("  Creating /etc/resolver/test ... ")
	if err := platform.WriteResolverFile(); err != nil {
		return err
	}
	fmt.Println("done")

	fmt.Print("  Starting daemon ... ")
	if err := platform.LoadAgent(); err != nil {
		return err
	}
	fmt.Println("done")

	fmt.Println()
	fmt.Println(ui.SuccessStyle.Render("  Outport is now routing *.test domains."))
	fmt.Println("  Run \"outport apply\" in any project to assign hostnames.")
	return nil
}

func runTeardown(cmd *cobra.Command, args []string) error {
	fmt.Print("  Stopping daemon ... ")
	platform.UnloadAgent() // best effort
	fmt.Println("done")

	fmt.Print("  Removing LaunchAgent ... ")
	platform.RemovePlist() // best effort
	fmt.Println("done")

	fmt.Print("  Removing /etc/resolver/test ... ")
	if err := platform.RemoveResolverFile(); err != nil {
		return err
	}
	fmt.Println("done")

	fmt.Println()
	fmt.Println("  Outport DNS and proxy removed.")
	return nil
}

func checkPort80() error {
	// Cannot bind port 80 directly (requires root). Use lsof to check.
	out, err := exec.Command("lsof", "-i", ":80", "-sTCP:LISTEN", "-t").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return fmt.Errorf("port 80 is in use (PID: %s). Stop the conflicting process and re-run `outport setup`",
			strings.TrimSpace(string(out)))
	}
	return nil
}
```

- [ ] **Step 2: Implement up/down commands**

```go
package cmd

import (
	"fmt"

	"github.com/outport-app/outport/internal/platform"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the Outport daemon",
	RunE:  runUp,
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the Outport daemon",
	RunE:  runDown,
}

func init() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	if !platform.IsSetup() {
		return fmt.Errorf("Outport is not set up. Run `outport setup` first.")
	}
	if err := platform.LoadAgent(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	fmt.Println("Outport daemon started.")
	return nil
}

func runDown(cmd *cobra.Command, args []string) error {
	if err := platform.UnloadAgent(); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	fmt.Println("Outport daemon stopped.")
	return nil
}
```

- [ ] **Step 3: Run build to verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/setup.go cmd/updown.go
git commit -m "feat: add setup, teardown, up, and down commands"
```

### Task 19: Add rename and promote commands

**Files:**
- Create: `cmd/rename.go`
- Create: `cmd/promote.go`
- Test: `cmd/cmd_test.go`

- [ ] **Step 1: Implement rename command**

```go
package cmd

import (
	"fmt"

	"github.com/outport-app/outport/internal/instance"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename an instance of the current project",
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	if err := instance.ValidateName(newName); err != nil {
		return err
	}

	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	oldKey := ctx.Cfg.Name + "/" + oldName
	newKey := ctx.Cfg.Name + "/" + newName

	// Check old exists
	alloc, ok := ctx.Reg.Projects[oldKey]
	if !ok {
		return fmt.Errorf("instance %q not found for project %q", oldName, ctx.Cfg.Name)
	}

	// Check new doesn't exist
	if _, exists := ctx.Reg.Projects[newKey]; exists {
		return fmt.Errorf("instance %q already exists for project %q", newName, ctx.Cfg.Name)
	}

	// Move the allocation
	delete(ctx.Reg.Projects, oldKey)

	// Recompute hostnames
	alloc.Hostnames = computeHostnames(ctx.Cfg, newName)
	ctx.Reg.Set(ctx.Cfg.Name, newName, alloc)

	if err := ctx.Reg.Save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	// Re-merge .env files for this instance
	templateVars := buildTemplateVars(ctx.Cfg, alloc.Ports, alloc.Hostnames)
	resolved := resolveDerivedFromAlloc(ctx.Cfg, alloc.Ports, templateVars)
	envFileVars := buildEnvFileVars(ctx.Cfg, alloc.Ports, resolved)
	for file, vars := range envFileVars {
		dotenv.Merge(filepath.Join(alloc.ProjectDir, file), vars)
	}

	fmt.Printf("Renamed %s → %s\n", oldName, newName)
	if alloc.Hostnames != nil {
		for _, h := range alloc.Hostnames {
			fmt.Printf("  %s\n", h)
		}
	}
	return nil
}
```

Note: The exact signatures for `buildTemplateVars`, `resolveDerivedFromAlloc`, and `buildEnvFileVars` will depend on how they are refactored in Task 8. The helper functions may need to be extracted from `runApply` to be reusable. Adapt during implementation.

- [ ] **Step 2: Implement promote command**

```go
package cmd

import (
	"fmt"

	"github.com/outport-app/outport/internal/instance"
	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote the current instance to main",
	RunE:  runPromote,
}

func init() {
	rootCmd.AddCommand(promoteCmd)
}

func runPromote(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	if ctx.Instance == "main" {
		return fmt.Errorf("already on the main instance")
	}

	mainKey := ctx.Cfg.Name + "/main"
	currentKey := ctx.Cfg.Name + "/" + ctx.Instance

	// Get current instance allocation
	currentAlloc, ok := ctx.Reg.Projects[currentKey]
	if !ok {
		return fmt.Errorf("instance %q not found", ctx.Instance)
	}

	// Handle existing main
	if mainAlloc, hasMain := ctx.Reg.Projects[mainKey]; hasMain {
		// Demote main to auto-generated code
		usedNames := make(map[string]bool)
		for key := range ctx.Reg.FindByProject(ctx.Cfg.Name) {
			parts := strings.SplitN(key, "/", 2)
			usedNames[parts[1]] = true
		}
		demotedName := instance.GenerateCode(usedNames)
		demotedKey := ctx.Cfg.Name + "/" + demotedName

		mainAlloc.Hostnames = computeHostnames(ctx.Cfg, demotedName)
		delete(ctx.Reg.Projects, mainKey)
		ctx.Reg.Set(ctx.Cfg.Name, demotedName, mainAlloc)

		fmt.Printf("Demoted main → %s\n", demotedName)

		// Re-merge demoted instance's .env files
		// (similar pattern to rename — recompute and write)
	}

	// Promote current to main
	currentAlloc.Hostnames = computeHostnames(ctx.Cfg, "main")
	delete(ctx.Reg.Projects, currentKey)
	ctx.Reg.Set(ctx.Cfg.Name, "main", currentAlloc)

	if err := ctx.Reg.Save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Promoted %s → main\n", ctx.Instance)
	// Re-merge promoted instance's .env files

	return nil
}
```

- [ ] **Step 3: Write tests for rename and promote**

Add tests to `cmd/cmd_test.go` covering:
- Rename succeeds and changes hostname
- Rename fails on collision
- Rename fails on invalid name
- Promote swaps main and current
- Promote works when no main exists
- Promote fails when already main

- [ ] **Step 4: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

- [ ] **Step 5: Run linter**

Run: `just lint`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/rename.go cmd/promote.go cmd/cmd_test.go
git commit -m "feat: add rename and promote commands for instance management"
```

---

## Chunk 9: Integration Testing and Polish

End-to-end testing, linting, documentation updates.

### Task 20: End-to-end integration tests

**Files:**
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Write integration test for full apply → hostnames flow**

Test the complete workflow:
1. Create a project with multiple HTTP services
2. Run `outport apply`
3. Verify registry contains hostnames and protocols
4. Run `outport apply` from a second directory (same project name)
5. Verify auto-generated instance code
6. Verify distinct hostnames
7. Verify `--json` output includes hostnames

- [ ] **Step 2: Write integration test for rename flow**

1. Set up two instances
2. Rename the non-main instance
3. Verify registry keys changed
4. Verify hostnames updated

- [ ] **Step 3: Write integration test for promote flow**

1. Set up main + non-main instance
2. Promote non-main
3. Verify main swapped
4. Verify hostnames correct

- [ ] **Step 4: Write integration test for template modifiers**

1. Config with `${service.url}` and `${service.url:direct}` in derived values
2. Run apply
3. Read .env file and verify resolved values

- [ ] **Step 5: Run all tests**

Run: `just test`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: add integration tests for instance model and hostname features"
```

### Task 21: Update CLAUDE.md, README, init presets

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `cmd/init.go`

- [ ] **Step 1: Update CLAUDE.md**

- Update Architecture section: replace worktree package with instance package, add daemon and platform packages
- Update CLI commands list: add setup, teardown, up, down, rename, promote
- Update Key Design Decisions: instance model, registry as source of truth, launchd socket activation
- Note the template modifier system

- [ ] **Step 2: Update README.md**

- Add documentation for `.test` domain setup
- Document `outport setup` / `outport teardown`
- Document instance management (`rename`, `promote`)
- Update config example to show `hostname` field
- Document template modifiers (`${service.url}`, `${service.url:direct}`)
- Update the "Planned" items in the comparison table to reflect what's now implemented

- [ ] **Step 3: Update init.go presets**

Add `hostname` field to the init template so new projects get it as a commented example:

```yaml
# hostname: myapp    # .test hostname (requires protocol: http/https)
```

- [ ] **Step 4: Run finalize checklist**

- `just lint` passes
- `just test` passes
- README commands list matches actual commands
- `init` presets include hostname field
- `--json` output works for changed commands
- CLAUDE.md reflects architectural changes

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md README.md cmd/init.go
git commit -m "docs: update CLAUDE.md, README, and init presets for .test domains"
```

### Task 22: Final lint and test pass

- [ ] **Step 1: Run linter**

Run: `just lint`
Expected: PASS — fix any issues.

- [ ] **Step 2: Run full test suite**

Run: `just test`
Expected: All PASS

- [ ] **Step 3: Build binary**

Run: `just build`
Expected: Binary compiles to `dist/outport`.

- [ ] **Step 4: Smoke test**

Run the binary manually:
```bash
dist/outport --version
dist/outport apply --help
dist/outport setup --help
dist/outport rename --help
dist/outport promote --help
```

- [ ] **Step 5: Commit any fixes**

If any fixes were needed from lint/test/smoke, commit them.

---

## Implementation Notes

### launchd Socket Activation (cgo) — Required for Port 80

The `launch_activate_socket()` call requires cgo and **cannot be deferred**. The daemon runs as the current user (not root), so it cannot bind port 80 directly. launchd socket activation is the mechanism that makes this work — launchd (root) binds the socket and passes the file descriptor to the unprivileged daemon.

**Development strategy:**
1. Build and test the daemon on a high port (e.g., 18080) first — all proxy/DNS logic works without cgo
2. Implement `launch_activate_socket()` via cgo using puma-dev's `cmd/puma-dev/main_darwin.go` as reference
3. The daemon accepts either a `--port` flag (for development) or a launchd socket (for production)
4. GoReleaser config needs `CGO_ENABLED=1` for macOS targets; Linux builds remain pure Go

### Task Dependencies

Tasks are ordered to build incrementally:
- Chunks 1-2 (registry, instance) have no external dependencies
- Chunk 3 (config) depends on chunks 1-2
- Chunk 4 (apply rework) depends on chunks 1-3
- Chunks 5-7 (daemon) can be worked on in parallel with chunk 4 since they only depend on chunks 1-2
- Chunk 8 (platform/CLI) depends on chunks 5-7
- Chunk 9 (integration/polish) depends on everything

### `--json` Support for New Commands

All new commands (setup, teardown, up, down, rename, promote) must support `--json` output following the existing pattern of paired `print*Styled()` / `print*JSON()` functions. The plan's code snippets show only styled output for brevity — the implementer must add JSON output paths checking `jsonFlag` in each command's `RunE` function.

### Shared `.env` Re-merge Helper

The rename, promote, and apply commands all need to recompute template vars, resolve derived values, and re-merge `.env` files. Extract a shared helper (e.g., `remergeEnvFiles(cfg, alloc, instanceName)`) to avoid duplication. The promote command's .env re-merge (for both the promoted and demoted instances) must be fully implemented, not left as a TODO comment.

### Breaking Changes

This implementation makes several breaking changes (all acceptable per Steve's guidance):
- Registry format adds `hostnames` and `protocols` fields
- `hostname` field in `.outport.yml` changes from full hostname (e.g., `myapp.localhost`) to stem (e.g., `myapp`)
- `internal/worktree` package removed entirely
- Instance names sourced from registry, not git worktree detection
- `${service.hostname}` template resolves to `.test` hostname instead of raw config value

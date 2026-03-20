# Config Evolution: Groups, Protocol, Multi-Env, Preferred Port

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evolve outport.yml to support `preferred_port` (try first, hash fallback), explicit `protocol`, `groups` with shared `env_file`, per-service `env_file` (string or array for multi-file writes), and config validation — while keeping simple configs unchanged.

**Architecture:** Config package gains a normalization step that flattens groups into the Services map with resolved fields. The allocator gains a preferred-port-first strategy. The `up` command groups services by env_file path and writes once per unique file. Output shows URLs for HTTP services and group headers when present. New validation catches env_var collisions, missing env_var, and empty groups.

**Tech Stack:** Go 1.24+, existing packages, lipgloss for output.

---

## File Structure

```
internal/
├── config/
│   ├── config.go          # MODIFY: New structs (Group, EnvFiles), normalization, validation
│   └── config_test.go     # MODIFY: Tests for groups, env_file arrays, validation
├── allocator/
│   ├── allocator.go       # MODIFY: Add preferred port support
│   └── allocator_test.go  # MODIFY: Tests for preferred port
├── ui/
│   └── styles.go          # MODIFY: Add GroupStyle, UrlStyle
cmd/
├── up.go                  # MODIFY: Multi-env-file writing, group-aware output, URLs
├── ports.go               # MODIFY: Group-aware output, URLs
├── init.go                # MODIFY: Use preferred_port in presets
```

---

## Chunk 1: Core Package Changes

### Task 1: Allocator — Preferred Port Support

**Files:**
- Modify: `internal/allocator/allocator.go`
- Modify: `internal/allocator/allocator_test.go`

The allocator gains a `preferred` parameter. If preferred > 0 and not taken, use it. Otherwise fall back to hash-based allocation.

- [ ] **Step 1: Write failing tests**

Add to `internal/allocator/allocator_test.go`:

```go
func TestAllocate_PreferredPortAvailable(t *testing.T) {
	port, err := Allocate("myapp", "main", "web", 3000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3000 {
		t.Errorf("port = %d, want 3000 (preferred port was available)", port)
	}
}

func TestAllocate_PreferredPortTaken(t *testing.T) {
	usedPorts := map[int]bool{3000: true}
	port, err := Allocate("myapp", "main", "web", 3000, usedPorts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 3000 {
		t.Error("should not have used taken preferred port")
	}
	// Should fall back to hash-based
	expected := HashPort("myapp", "main", "web")
	if port != expected {
		// Could be expected or probed from expected — just check range
		if port < MinPort || port > MaxPort {
			t.Errorf("port %d outside range", port)
		}
	}
}

func TestAllocate_PreferredPortZero(t *testing.T) {
	// preferred=0 means no preference, use hash
	port, err := Allocate("myapp", "main", "web", 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := HashPort("myapp", "main", "web")
	if port != expected {
		t.Errorf("port = %d, want %d (no preferred, should use hash)", port, expected)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/allocator/ -v -run TestAllocate_Preferred
```

Expected: Compilation error — `Allocate` signature changed.

- [ ] **Step 3: Update allocator.go**

Change `Allocate` signature to accept `preferred int`:

```go
// Allocate returns a port for the given service. If preferred > 0 and available, uses it.
// Otherwise falls back to hash-based allocation with linear probing.
func Allocate(project, instance, service string, preferred int, usedPorts map[int]bool) (int, error) {
	if preferred > 0 && !usedPorts[preferred] {
		return preferred, nil
	}
	start := HashPort(project, instance, service)
	port := start
	for usedPorts[port] {
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

- [ ] **Step 4: Update existing tests to pass preferred=0**

Update all existing `Allocate` calls in `allocator_test.go` to pass `0` as the preferred parameter:

- `TestAllocate_NoCollisions`: `Allocate("myapp", "main", "web", 0, nil)`
- `TestAllocate_WithCollision`: `Allocate("myapp", "main", "web", 0, usedPorts)`
- `TestAllocate_WithMultipleCollisions`: `Allocate("myapp", "main", "web", 0, usedPorts)`

- [ ] **Step 5: Update cmd/up.go call site**

Change the `Allocate` call in `cmd/up.go` to pass `svc.DefaultPort` (will be renamed to `PreferredPort` in Task 2, but for now pass the existing field to keep it compiling):

```go
port, err := allocator.Allocate(cfg.Name, wt.Instance, svcName, svc.DefaultPort, usedPorts)
```

- [ ] **Step 6: Run all tests**

```bash
go test ./... -v
```

Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/allocator/ cmd/up.go
git commit -m "feat: allocator tries preferred port first, hash fallback"
```

---

### Task 2: Config Struct Evolution

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

Major changes:
- `DefaultPort` → `PreferredPort` (yaml: `preferred_port`)
- Add `Protocol string` (explicit, optional)
- Add `EnvFiles` (string or []string from yaml, resolved to []string)
- Add `Group string` (set during normalization)
- Add `Group` struct with `EnvFile` and `Services`
- Add `Groups` to `Config`
- Normalization flattens groups, resolves defaults, validates

- [ ] **Step 1: Write failing tests**

Replace `internal/config/config_test.go` entirely:

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "outport.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad_SimpleConfig(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("name = %q, want %q", cfg.Name, "myapp")
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	web := cfg.Services["web"]
	if web.PreferredPort != 3000 {
		t.Errorf("web.PreferredPort = %d, want 3000", web.PreferredPort)
	}
	if web.EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want %q", web.EnvVar, "PORT")
	}
	// Default env_file
	if len(web.EnvFiles) != 1 || web.EnvFiles[0] != ".env" {
		t.Errorf("web.EnvFiles = %v, want [.env]", web.EnvFiles)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

func TestLoad_MissingName(t *testing.T) {
	dir := writeConfig(t, `services:
  web:
    preferred_port: 3000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoad_NoServices(t *testing.T) {
	dir := writeConfig(t, `name: myapp
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for no services, got nil")
	}
}

func TestLoad_WithProtocol(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Protocol != "http" {
		t.Errorf("web.Protocol = %q, want %q", cfg.Services["web"].Protocol, "http")
	}
	if cfg.Services["postgres"].Protocol != "" {
		t.Errorf("postgres.Protocol = %q, want empty", cfg.Services["postgres"].Protocol)
	}
}

func TestLoad_WithEnvFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  rails:
    preferred_port: 3000
    env_var: RAILS_PORT
    env_file: backend/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services["web"].EnvFiles) != 1 || cfg.Services["web"].EnvFiles[0] != ".env" {
		t.Errorf("web.EnvFiles = %v, want [.env]", cfg.Services["web"].EnvFiles)
	}
	if len(cfg.Services["rails"].EnvFiles) != 1 || cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles = %v, want [backend/.env]", cfg.Services["rails"].EnvFiles)
	}
}

func TestLoad_WithEnvFileArray(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  postgres:
    preferred_port: 5432
    env_var: DB_PORT
    env_file:
      - backend/.env
      - .env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pg := cfg.Services["postgres"]
	if len(pg.EnvFiles) != 2 {
		t.Fatalf("postgres.EnvFiles count = %d, want 2", len(pg.EnvFiles))
	}
	if pg.EnvFiles[0] != "backend/.env" || pg.EnvFiles[1] != ".env" {
		t.Errorf("postgres.EnvFiles = %v, want [backend/.env, .env]", pg.EnvFiles)
	}
}

func TestLoad_WithGroups(t *testing.T) {
	dir := writeConfig(t, `name: unio
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
      postgres:
        preferred_port: 5432
        env_var: DB_PORT
  frontend:
    services:
      main:
        preferred_port: 9000
        env_var: MAIN_PORT
      portal:
        preferred_port: 9001
        env_var: PORTAL_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 4 {
		t.Fatalf("services count = %d, want 4", len(cfg.Services))
	}
	// Backend inherits group env_file
	if cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles = %v, want [backend/.env]", cfg.Services["rails"].EnvFiles)
	}
	// Frontend defaults to .env
	if cfg.Services["main"].EnvFiles[0] != ".env" {
		t.Errorf("main.EnvFiles = %v, want [.env]", cfg.Services["main"].EnvFiles)
	}
	// Group names set
	if cfg.Services["rails"].Group != "backend" {
		t.Errorf("rails.Group = %q, want %q", cfg.Services["rails"].Group, "backend")
	}
	if cfg.Services["main"].Group != "frontend" {
		t.Errorf("main.Group = %q, want %q", cfg.Services["main"].Group, "frontend")
	}
}

func TestLoad_MixedServicesAndGroups(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  lookbook:
    preferred_port: 4100
    env_var: LOOKBOOK_PORT
    protocol: http
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	if cfg.Services["lookbook"].Group != "" {
		t.Errorf("lookbook.Group = %q, want empty", cfg.Services["lookbook"].Group)
	}
	if cfg.Services["rails"].Group != "backend" {
		t.Errorf("rails.Group = %q, want %q", cfg.Services["rails"].Group, "backend")
	}
}

func TestLoad_PerServiceEnvFileOverridesGroup(t *testing.T) {
	dir := writeConfig(t, `name: myapp
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
      special:
        preferred_port: 4000
        env_var: SPECIAL_PORT
        env_file: special/.env
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["rails"].EnvFiles[0] != "backend/.env" {
		t.Errorf("rails.EnvFiles[0] = %q, want %q", cfg.Services["rails"].EnvFiles[0], "backend/.env")
	}
	if cfg.Services["special"].EnvFiles[0] != "special/.env" {
		t.Errorf("special.EnvFiles[0] = %q, want %q", cfg.Services["special"].EnvFiles[0], "special/.env")
	}
}

// Validation tests

func TestLoad_DuplicateServiceName(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
groups:
  frontend:
    services:
      web:
        preferred_port: 4000
        env_var: WEB_PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate service name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}

func TestLoad_MissingEnvVar(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing env_var, got nil")
	}
	if !strings.Contains(err.Error(), "env_var") {
		t.Errorf("error = %q, want to contain 'env_var'", err.Error())
	}
}

func TestLoad_EnvVarCollisionSameFile(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 4000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for env_var collision, got nil")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error = %q, want to contain 'PORT'", err.Error())
	}
}

func TestLoad_EnvVarSameNameDifferentFiles(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 4000
    env_var: PORT
    env_file: backend/.env
`)
	// Same env_var in different files is OK
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v (same env_var in different files should be allowed)", err)
	}
}

func TestLoad_EmptyGroup(t *testing.T) {
	dir := writeConfig(t, `name: myapp
groups:
  empty:
    env_file: backend/.env
services:
  web:
    preferred_port: 3000
    env_var: PORT
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for empty group, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want to contain 'empty'", err.Error())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/ -v
```

Expected: Compilation errors.

- [ ] **Step 3: Implement config.go**

Replace `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = "outport.yml"

// EnvFileField handles YAML that can be a string or []string.
type EnvFileField []string

func (e *EnvFileField) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*e = []string{value.Value}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*e = list
		return nil
	}
	return fmt.Errorf("env_file must be a string or list of strings")
}

type Service struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Protocol      string       `yaml:"protocol"`
	RawEnvFile    EnvFileField `yaml:"env_file"`
	EnvFiles      []string     `yaml:"-"` // resolved during normalization
	Group         string       `yaml:"-"` // set during normalization
}

type Group struct {
	EnvFile  string             `yaml:"env_file"`
	Services map[string]Service `yaml:"services"`
}

type Config struct {
	Name     string             `yaml:"name"`
	Services map[string]Service `yaml:"services"`
	Groups   map[string]Group   `yaml:"groups"`
}

func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("config: 'name' is required")
	}

	if err := cfg.normalize(); err != nil {
		return nil, err
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("config: at least one service is required")
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) normalize() error {
	if c.Services == nil {
		c.Services = make(map[string]Service)
	}

	// Validate groups are not empty
	for groupName, group := range c.Groups {
		if len(group.Services) == 0 {
			return fmt.Errorf("config: group %q has no services", groupName)
		}
	}

	// Flatten group services into top-level Services
	for groupName, group := range c.Groups {
		for svcName, svc := range group.Services {
			if _, exists := c.Services[svcName]; exists {
				return fmt.Errorf("config: duplicate service name %q", svcName)
			}
			// Resolve env_file: service-level > group-level > default
			if len(svc.RawEnvFile) == 0 && group.EnvFile != "" {
				svc.RawEnvFile = EnvFileField{group.EnvFile}
			}
			svc.Group = groupName
			c.Services[svcName] = svc
		}
	}

	// Resolve defaults for all services
	for name, svc := range c.Services {
		if len(svc.RawEnvFile) == 0 {
			svc.EnvFiles = []string{".env"}
		} else {
			svc.EnvFiles = []string(svc.RawEnvFile)
		}
		c.Services[name] = svc
	}

	return nil
}

func (c *Config) validate() error {
	// Check for missing env_var and env_var collisions within same file
	fileVars := make(map[string]map[string]string) // envFile -> envVar -> serviceName

	for name, svc := range c.Services {
		if svc.EnvVar == "" {
			return fmt.Errorf("config: service %q is missing required field 'env_var'", name)
		}
		for _, envFile := range svc.EnvFiles {
			if fileVars[envFile] == nil {
				fileVars[envFile] = make(map[string]string)
			}
			if other, exists := fileVars[envFile][svc.EnvVar]; exists {
				return fmt.Errorf("config: services %q and %q both write %s to %s",
					other, name, svc.EnvVar, envFile)
			}
			fileVars[envFile][svc.EnvVar] = name
		}
	}

	return nil
}
```

- [ ] **Step 4: Run config tests**

```bash
go test ./internal/config/ -v
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: evolve config with groups, preferred_port, protocol, multi-env-file"
```

---

### Task 3: Update UI Styles

**Files:**
- Modify: `internal/ui/styles.go`

- [ ] **Step 1: Add GroupStyle and UrlStyle**

Add to the `var` block in `internal/ui/styles.go`:

```go
	// Group header in output
	GroupStyle = lipgloss.NewStyle().
		Foreground(Purple).
		Bold(true)

	// URL for HTTP services
	UrlStyle = lipgloss.NewStyle().
		Foreground(Yellow)
```

- [ ] **Step 2: Commit**

```bash
git add internal/ui/styles.go
git commit -m "feat: add GroupStyle and UrlStyle"
```

---

## Chunk 2: Command Updates

### Task 4: Update `outport up` — Multi-Env, Groups, URLs

**Files:**
- Modify: `cmd/up.go`

Key changes:
- Use `svc.PreferredPort` in `Allocate` call
- Build `map[string]map[string]string` (envFile → vars) instead of flat map
- Write to multiple env files
- Show URLs for HTTP services
- Show group headers when groups present

- [ ] **Step 1: Rewrite cmd/up.go**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Allocate ports and write to .env files",
	Long:  "Reads outport.yml, allocates deterministic ports, and writes them to .env files.",
	RunE:  runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(dir)
	if err != nil {
		return fmt.Errorf("detecting worktree: %w", err)
	}

	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	existing, hasExisting := reg.Get(cfg.Name, wt.Instance)

	usedPorts := reg.UsedPorts()
	if hasExisting {
		for _, port := range existing.Ports {
			delete(usedPorts, port)
		}
	}

	ports := make(map[string]int)
	// Group env vars by target file
	envFileVars := make(map[string]map[string]string)

	serviceNames := sortedServiceNames(cfg)

	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		var port int

		// Reuse existing allocation
		if hasExisting {
			if existingPort, ok := existing.Ports[svcName]; ok {
				port = existingPort
				usedPorts[existingPort] = true
			}
		}

		// Allocate new port (preferred first, hash fallback)
		if port == 0 {
			var err error
			port, err = allocator.Allocate(cfg.Name, wt.Instance, svcName, svc.PreferredPort, usedPorts)
			if err != nil {
				return fmt.Errorf("allocating port for %s: %w", svcName, err)
			}
			usedPorts[port] = true
		}
		ports[svcName] = port

		// Add to each target env file
		for _, envFile := range svc.EnvFiles {
			if envFileVars[envFile] == nil {
				envFileVars[envFile] = make(map[string]string)
			}
			envFileVars[envFile][svc.EnvVar] = fmt.Sprintf("%d", port)
		}
	}

	// Save registry
	reg.Set(cfg.Name, wt.Instance, registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
	})
	if err := reg.Save(); err != nil {
		return err
	}

	// Write each env file
	envFiles := sortedKeys(envFileVars)
	for _, envFile := range envFiles {
		envPath := filepath.Join(dir, envFile)
		if err := dotenv.Merge(envPath, envFileVars[envFile]); err != nil {
			return fmt.Errorf("writing %s: %w", envFile, err)
		}
	}

	// Output
	if jsonFlag {
		return printUpJSON(cmd, cfg, wt, ports, envFiles)
	}
	return printUpStyled(cmd, cfg, wt, serviceNames, ports, envFiles)
}

func sortedServiceNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// JSON output types

type svcJSON struct {
	Port          int      `json:"port"`
	PreferredPort int      `json:"preferred_port,omitempty"`
	EnvVar        string   `json:"env_var"`
	Protocol      string   `json:"protocol,omitempty"`
	URL           string   `json:"url,omitempty"`
	EnvFiles      []string `json:"env_files"`
	Group         string   `json:"group,omitempty"`
}

type upJSON struct {
	Project  string             `json:"project"`
	Instance string             `json:"instance"`
	Services map[string]svcJSON `json:"services"`
	EnvFiles []string           `json:"env_files"`
}

func printUpJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, ports map[string]int, envFiles []string) error {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		s := svcJSON{
			Port:          ports[name],
			PreferredPort: svc.PreferredPort,
			EnvVar:        svc.EnvVar,
			Protocol:      svc.Protocol,
			EnvFiles:      svc.EnvFiles,
			Group:         svc.Group,
		}
		if svc.Protocol == "http" || svc.Protocol == "https" {
			s.URL = fmt.Sprintf("%s://localhost:%d", svc.Protocol, ports[name])
		}
		services[name] = s
	}
	out := upJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: services,
		EnvFiles: envFiles,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printUpStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, serviceNames []string, ports map[string]int, envFiles []string) error {
	w := cmd.OutOrStdout()

	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}

	header := ui.ProjectStyle.Render(cfg.Name) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)

	// Check if any services have groups
	hasGroups := false
	for _, svcName := range serviceNames {
		if cfg.Services[svcName].Group != "" {
			hasGroups = true
			break
		}
	}

	if hasGroups {
		printGroupedServices(w, cfg, serviceNames, ports)
	} else {
		printFlatServices(w, cfg, serviceNames, ports)
	}

	lipgloss.Fprintln(w)
	if len(envFiles) == 1 {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("Ports written to "+envFiles[0]))
	} else {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("Ports written to:"))
		for _, f := range envFiles {
			lipgloss.Fprintln(w, ui.SuccessStyle.Render("  "+f))
		}
	}
	return nil
}

func printGroupedServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int) {
	var ungrouped []string
	groupServices := make(map[string][]string)
	var groupOrder []string

	for _, svcName := range serviceNames {
		group := cfg.Services[svcName].Group
		if group == "" {
			ungrouped = append(ungrouped, svcName)
		} else {
			if _, seen := groupServices[group]; !seen {
				groupOrder = append(groupOrder, group)
			}
			groupServices[group] = append(groupServices[group], svcName)
		}
	}
	sort.Strings(groupOrder)

	for _, svcName := range ungrouped {
		printServiceLine(w, cfg, svcName, ports[svcName])
	}
	if len(ungrouped) > 0 && len(groupOrder) > 0 {
		lipgloss.Fprintln(w)
	}

	for i, group := range groupOrder {
		lipgloss.Fprintln(w, "  "+ui.GroupStyle.Render(group))
		for _, svcName := range groupServices[group] {
			printServiceLine(w, cfg, svcName, ports[svcName])
		}
		if i < len(groupOrder)-1 {
			lipgloss.Fprintln(w)
		}
	}
}

func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int) {
	for _, svcName := range serviceNames {
		printServiceLine(w, cfg, svcName, ports[svcName])
	}
}

func printServiceLine(w io.Writer, cfg *config.Config, svcName string, port int) {
	svc := cfg.Services[svcName]
	portDisplay := ui.PortStyle.Render(fmt.Sprintf("%d", port))
	if svc.Protocol == "http" || svc.Protocol == "https" {
		portDisplay = ui.UrlStyle.Render(fmt.Sprintf("%s://localhost:%d", svc.Protocol, port))
	}
	line := fmt.Sprintf("    %s  %s  %s %s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", svc.EnvVar)),
		ui.Arrow,
		portDisplay,
	)
	lipgloss.Fprintln(w, line)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build -o dist/outport .
```

- [ ] **Step 3: Commit**

```bash
git add cmd/up.go
git commit -m "feat: multi-env-file writing, groups, URLs in outport up"
```

---

### Task 5: Update `outport ports`

**Files:**
- Modify: `cmd/ports.go`

Mirror the group-aware output and URL display from `outport up`.

- [ ] **Step 1: Rewrite cmd/ports.go**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show ports for the current project",
	RunE:  runPorts,
}

func init() {
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(dir)
	if err != nil {
		return err
	}

	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	alloc, ok := reg.Get(cfg.Name, wt.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	if jsonFlag {
		return printPortsJSON(cmd, cfg, wt, alloc)
	}
	return printPortsStyled(cmd, cfg, wt, alloc)
}

func printPortsJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, alloc registry.Allocation) error {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		port := alloc.Ports[name]
		s := svcJSON{
			Port:     port,
			EnvVar:   svc.EnvVar,
			Protocol: svc.Protocol,
			EnvFiles: svc.EnvFiles,
			Group:    svc.Group,
		}
		if svc.Protocol == "http" || svc.Protocol == "https" {
			s.URL = fmt.Sprintf("%s://localhost:%d", svc.Protocol, port)
		}
		services[name] = s
	}
	out := upJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: services,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printPortsStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, alloc registry.Allocation) error {
	w := cmd.OutOrStdout()

	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}

	header := ui.ProjectStyle.Render(cfg.Name) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)

	serviceNames := make([]string, 0, len(alloc.Ports))
	for s := range alloc.Ports {
		serviceNames = append(serviceNames, s)
	}
	sort.Strings(serviceNames)

	hasGroups := false
	for _, svcName := range serviceNames {
		if svc, ok := cfg.Services[svcName]; ok && svc.Group != "" {
			hasGroups = true
			break
		}
	}

	if hasGroups {
		printGroupedServices(w, cfg, serviceNames, alloc.Ports)
	} else {
		printFlatServices(w, cfg, serviceNames, alloc.Ports)
	}

	return nil
}
```

- [ ] **Step 2: Verify build**

```bash
go build -o dist/outport .
```

- [ ] **Step 3: Commit**

```bash
git add cmd/ports.go
git commit -m "feat: group-aware output and URLs in outport ports"
```

---

### Task 6: Update `outport init` Presets

**Files:**
- Modify: `cmd/init.go`

Change `DefaultPort` → `PreferredPort` in preset struct and YAML output. Add `protocol: http` to web/vite presets.

- [ ] **Step 1: Update init.go**

Change the `servicePreset` struct field from `DefaultPort` to `PreferredPort`. Add `Protocol` field. Update the YAML output to use `preferred_port` and include `protocol` when set.

```go
type servicePreset struct {
	Name          string
	PreferredPort int
	EnvVar        string
	Protocol      string
}

var presets = []servicePreset{
	{"web", 3000, "PORT", "http"},
	{"postgres", 5432, "DATABASE_PORT", ""},
	{"redis", 6379, "REDIS_PORT", ""},
	{"mailpit_web", 8025, "MAILPIT_WEB_PORT", "http"},
	{"mailpit_smtp", 1025, "MAILPIT_SMTP_PORT", ""},
	{"vite", 5173, "VITE_PORT", "http"},
}
```

Update the YAML generation:

```go
	for _, svc := range selectedServices {
		sb.WriteString(fmt.Sprintf("  %s:\n", svc.Name))
		sb.WriteString(fmt.Sprintf("    preferred_port: %d\n", svc.PreferredPort))
		sb.WriteString(fmt.Sprintf("    env_var: %s\n", svc.EnvVar))
		if svc.Protocol != "" {
			sb.WriteString(fmt.Sprintf("    protocol: %s\n", svc.Protocol))
		}
	}
```

Update the prompt line to show `preferred_port`:

```go
fmt.Fprintf(cmd.OutOrStdout(), "  %s (preferred port %d)? [y/N]: ", preset.Name, preset.PreferredPort)
```

- [ ] **Step 2: Verify build and tests**

```bash
go build -o dist/outport . && go test ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "feat: update init presets with preferred_port and protocol"
```

---

### Task 7: Manual Integration Tests

- [ ] **Step 1: Test simple config**

```bash
cd /tmp && rm -rf outport-test-simple && mkdir outport-test-simple && cd outport-test-simple && git init -q
cat > outport.yml << 'EOF'
name: simple-app
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
EOF
/Users/steve/src/outport-app/dist/outport up
cat .env
/Users/steve/src/outport-app/dist/outport ports --json
```

Expected: Web gets 3000 (preferred, available), postgres gets 5432. URLs shown for web. JSON includes protocol and url fields.

- [ ] **Step 2: Test grouped monorepo config**

```bash
cd /tmp && rm -rf outport-test-mono && mkdir -p outport-test-mono/backend && cd outport-test-mono && git init -q
cat > outport.yml << 'EOF'
name: unio
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
        protocol: http
      postgres:
        preferred_port: 5432
        env_var: DB_PORT
        env_file:
          - backend/.env
          - .env
      redis:
        preferred_port: 6379
        env_var: REDIS_PORT
  frontend:
    services:
      main:
        preferred_port: 9000
        env_var: MAIN_PORT
        protocol: http
      portal:
        preferred_port: 9001
        env_var: PORTAL_PORT
        protocol: http
EOF
/Users/steve/src/outport-app/dist/outport up
echo "--- root .env ---"
cat .env
echo "--- backend/.env ---"
cat backend/.env
```

Expected: Two env files written. `backend/.env` has RAILS_PORT, DB_PORT, REDIS_PORT. Root `.env` has MAIN_PORT, PORTAL_PORT, DB_PORT. Group headers in output.

- [ ] **Step 3: Test preferred port collision**

```bash
cd /tmp && rm -rf outport-test-collision && mkdir outport-test-collision && cd outport-test-collision && git init -q
cat > outport.yml << 'EOF'
name: collision-test
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 3000
    env_var: API_PORT
EOF
/Users/steve/src/outport-app/dist/outport up
```

Expected: One gets 3000, the other falls back to hash-based allocation.

- [ ] **Step 4: Test validation errors**

```bash
cd /tmp && rm -rf outport-test-val && mkdir outport-test-val && cd outport-test-val && git init -q
# Test env_var collision
cat > outport.yml << 'EOF'
name: val-test
services:
  web:
    preferred_port: 3000
    env_var: PORT
  api:
    preferred_port: 4000
    env_var: PORT
EOF
/Users/steve/src/outport-app/dist/outport up 2>&1 || true
```

Expected: Error about env_var PORT collision in .env.

- [ ] **Step 5: Clean up**

```bash
rm -rf /tmp/outport-test-simple /tmp/outport-test-mono /tmp/outport-test-collision /tmp/outport-test-val
```

- [ ] **Step 6: Run full test suite**

```bash
cd /Users/steve/src/outport-app && go test ./... -v
```

Expected: All tests PASS.

---

## Summary

| Change | What | Why |
|--------|------|-----|
| `default_port` → `preferred_port` | Allocator tries preferred first, hash fallback | Honest name, does what users expect |
| `protocol` | Explicit, optional (no inference) | Fragile inference dropped per adversarial review |
| `groups` | Optional org concept with shared `env_file` | Monorepo structure, DRY for shared env_file |
| `env_file` string or array | Write same var to multiple files | Docker Compose bridge problem |
| Validation | env_var collision, missing env_var, empty groups | Prevent silent data loss |
| Output | URLs for http/https services, group headers | Better DX |

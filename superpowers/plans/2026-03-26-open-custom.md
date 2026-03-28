# Custom `open` Service List — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level `open` field to `outport.yml` so `outport open` only opens the listed services instead of all services with hostnames.

**Architecture:** New `Open []string` field on `rawConfig` and `Config`. Validation rejects unknown services, services without hostnames, and duplicates. `mergeLocal()` replaces the list entirely when the local file declares it. The `open` command checks `cfg.Open` before iterating.

**Tech Stack:** Go, YAML (`gopkg.in/yaml.v3`), Cobra CLI

---

### Task 1: Config — parse and normalize the `open` field

**Files:**
- Modify: `internal/config/config.go:300-303` (rawConfig struct)
- Modify: `internal/config/config.go:312-327` (Config struct)
- Modify: `internal/config/config.go:472-499` (normalize method)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test — valid `open` list is parsed**

Add to `internal/config/config_test.go`:

```go
// --- Open field ---

func TestLoad_OpenField(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp
  postgres:
    env_var: DB_PORT
open:
  - web
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Open) != 1 || cfg.Open[0] != "web" {
		t.Errorf("Open = %v, want [web]", cfg.Open)
	}
}

func TestLoad_OpenFieldAbsent(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Open != nil {
		t.Errorf("Open = %v, want nil", cfg.Open)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_OpenField" -v`
Expected: FAIL — `cfg.Open` field does not exist

- [ ] **Step 3: Add the `Open` field to `rawConfig` and `Config`, populate in `normalize()`**

In `internal/config/config.go`, add `Open` to `rawConfig`:

```go
// rawConfig is the YAML deserialization target.
type rawConfig struct {
	Name        string                      `yaml:"name"`
	Open        []string                    `yaml:"open"`
	RawServices map[string]rawService       `yaml:"services"`
	RawComputed map[string]rawComputedValue `yaml:"computed"`
}
```

Add `Open` to `Config`:

```go
type Config struct {
	// Name is the project identifier...
	Name string

	// Open is an optional list of service names that `outport open` should open
	// by default. When nil, all services with hostnames are opened. When non-nil,
	// only the listed services are opened. Order determines browser tab order.
	Open []string

	// Services maps service names...
	Services map[string]Service

	// Computed maps environment variable names...
	Computed map[string]ComputedValue
}
```

In the `normalize` method, after the existing body, add:

```go
func (c *Config) normalize(raw *rawConfig) error {
	// ... existing service and computed normalization ...

	c.Open = raw.Open

	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -run "TestLoad_OpenField" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: parse open field from outport.yml config"
```

---

### Task 2: Config — validate the `open` field

**Files:**
- Modify: `internal/config/config.go:501-617` (validate method)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for all three validation rules**

Add to `internal/config/config_test.go`:

```go
func TestLoad_OpenUnknownServiceErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
open:
  - web
  - missing
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown service in open, got nil")
	}
	if !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %q, want to contain '\"missing\"'", err.Error())
	}
}

func TestLoad_OpenServiceWithoutHostnameErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  postgres:
    env_var: DB_PORT
open:
  - web
  - postgres
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for service without hostname in open, got nil")
	}
	if !strings.Contains(err.Error(), `"postgres"`) {
		t.Errorf("error = %q, want to contain '\"postgres\"'", err.Error())
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error = %q, want to contain 'hostname'", err.Error())
	}
}

func TestLoad_OpenDuplicateErrors(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
open:
  - web
  - web
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for duplicate in open, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want to contain 'duplicate'", err.Error())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_Open.*(Unknown|WithoutHostname|Duplicate)" -v`
Expected: FAIL — no validation errors returned

- [ ] **Step 3: Add validation logic to `validate()`**

In `internal/config/config.go`, add this block at the end of the `validate()` method, just before the final `return nil`:

```go
	// Validate open list
	if c.Open != nil {
		seen := make(map[string]bool)
		for _, name := range c.Open {
			if seen[name] {
				return fmt.Errorf("open: duplicate entry %q", name)
			}
			seen[name] = true
			svc, ok := c.Services[name]
			if !ok {
				return fmt.Errorf("open: service %q does not exist in services", name)
			}
			if svc.Hostname == "" {
				return fmt.Errorf("open: service %q has no hostname", name)
			}
		}
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -run "TestLoad_Open" -v`
Expected: PASS (all six tests from Task 1 and Task 2)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: validate open field — unknown service, missing hostname, duplicates"
```

---

### Task 3: Config — local override support for `open`

**Files:**
- Modify: `internal/config/config.go:421-460` (mergeLocal function)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for local override**

Add to `internal/config/config_test.go`:

```go
func TestLoad_LocalOverridesOpen(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp
open:
  - web
  - admin
`)
	writeLocalConfig(t, dir, `open:
  - admin
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Open) != 1 || cfg.Open[0] != "admin" {
		t.Errorf("Open = %v, want [admin]", cfg.Open)
	}
}

func TestLoad_LocalAddsOpenWhenBaseHasNone(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp
`)
	writeLocalConfig(t, dir, `open:
  - web
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Open) != 1 || cfg.Open[0] != "web" {
		t.Errorf("Open = %v, want [web]", cfg.Open)
	}
}

func TestLoad_LocalWithoutOpenKeepsBase(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp
open:
  - web
  - admin
`)
	writeLocalConfig(t, dir, `services:
  web:
    preferred_port: 3000
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Open) != 2 {
		t.Errorf("Open = %v, want [web admin]", cfg.Open)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_Local.*(Open|Adds|Without)" -v`
Expected: FAIL — `TestLoad_LocalOverridesOpen` and `TestLoad_LocalAddsOpenWhenBaseHasNone` will fail because `mergeLocal` doesn't handle `Open` yet

- [ ] **Step 3: Add `Open` to `rawConfig` merge in `mergeLocal()`**

In `internal/config/config.go`, in the `mergeLocal()` function, add handling for `Open` after the service merge loop (before `return nil`):

```go
	if local.Open != nil {
		base.Open = local.Open
	}

	return nil
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -run "TestLoad_Local.*(Open|Adds|Without)" -v`
Expected: PASS

- [ ] **Step 5: Run all config tests to check for regressions**

Run: `go test ./internal/config/ -v`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: support open field in outport.local.yml overrides"
```

---

### Task 4: Open command — respect the `open` config field

**Files:**
- Modify: `cmd/open.go:47-64`

- [ ] **Step 1: Modify `runOpen` to use `cfg.Open` when present**

In `cmd/open.go`, replace the "open all" block (lines 47–64) with:

```go
	// Determine which services to open
	var serviceNames []string
	if len(ctx.Cfg.Open) > 0 {
		// Config specifies which services to open — use that order
		serviceNames = ctx.Cfg.Open
	} else {
		// No open list — open all services with hostnames (alphabetical)
		for _, name := range slices.Sorted(maps.Keys(ctx.Cfg.Services)) {
			if ctx.Cfg.Services[name].Hostname != "" {
				serviceNames = append(serviceNames, name)
			}
		}
	}

	opened := 0
	for _, svcName := range serviceNames {
		svc := ctx.Cfg.Services[svcName]
		h := svc.Hostname
		if allocated, ok := alloc.Hostnames[svcName]; ok {
			h = allocated
		}
		url := fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(h, httpsEnabled), h)
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open %s: %v\n", svcName, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", svcName, url)
		opened++
	}

	if opened == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No web services found. Add 'hostname' to services in outport.yml.")
	}
```

Note: the `svc.Hostname == ""` check is removed from the loop body because when `cfg.Open` is set, validation already guarantees all listed services have hostnames. When `cfg.Open` is nil, the filter above already excludes services without hostnames.

- [ ] **Step 2: Run linter and full test suite**

Run: `just lint && just test`
Expected: All pass

- [ ] **Step 3: Commit**

```bash
git add cmd/open.go
git commit -m "feat: outport open respects the open config field"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/commands.md`

- [ ] **Step 1: Add `open` field documentation to configuration.md**

In `docs/reference/configuration.md`, add a new section after `#### services (required)` (after line 60) and before `#### env_var (required)` (line 62):

```markdown
#### `open`

Declares which services `outport open` opens by default. When omitted, `outport open` opens all services with a `hostname`. When present, only the listed services are opened — in the order listed.

```yaml
name: myapp

open:
  - web
  - frontend

services:
  web:
    env_var: PORT
    hostname: myapp
  frontend:
    env_var: VITE_PORT
    hostname: app.myapp
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp    # not opened by default
```

Each entry must reference a service that exists and has a `hostname`. You can always open any service explicitly: `outport open admin`.

Can be overridden in `outport.local.yml` — the local list replaces the base list entirely.
```

- [ ] **Step 2: Update the `outport open` section in commands.md**

In `docs/reference/commands.md`, update the `outport open` section (around line 75) to mention the config field:

```markdown
### `outport open`

Open HTTP services in the browser.

```bash
outport open         # open default services (or all HTTP services)
outport open web     # open a specific service
```

Opens HTTP services in your default browser. By default, opens all services with a `hostname`. If the `open` field is set in `outport.yml`, only the listed services are opened. Specify a service name to open just that one, regardless of the `open` list.

Works best with `.test` domains set up (`outport system start`).
```

- [ ] **Step 3: Add `open` to the local overrides table**

In `docs/reference/configuration.md`, in the "Common Uses" table under "Local Overrides" (around line 255), add a row:

```markdown
| Only open specific services on this machine | `open: [web]` at the top level |
```

- [ ] **Step 4: Commit**

```bash
git add docs/reference/configuration.md docs/reference/commands.md
git commit -m "docs: document the open config field"
```

---

### Task 6: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `open` to the template expansion / design decisions section**

In `CLAUDE.md`, in the "Key Design Decisions" section, under the `.test hostnames` bullet, add a note about the `open` field:

```markdown
- **`open` list** — Optional top-level `open` field in `outport.yml` lists which services `outport open` opens by default. When absent, all services with hostnames are opened (original behavior). Validated: each entry must exist and have a hostname. Overridable in `outport.local.yml` (replaces entirely).
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add open field to CLAUDE.md design decisions"
```

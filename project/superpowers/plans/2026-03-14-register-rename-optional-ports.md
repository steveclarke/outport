# Register Rename + Optional Ports Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename `outport up` to `outport register` (alias `reg`), add `outport unregister`, remove `outport reset` (replaced by `--force` flag), and make `preferred_port` optional in config so new projects don't need to think about port numbers.

**Architecture:** CLI command rename with backward-compatible alias. Config parser already treats `preferred_port` as a zero-value int when omitted (Go default), so the allocator already handles it — just need to update `outport init` to stop generating it and update tests/docs.

**Tech Stack:** Go 1.24+, Cobra CLI, existing internal packages.

---

## File Structure

```
cmd/
├── register.go     # RENAME from up.go — same logic, new command name + alias
├── unregister.go   # NEW — remove project/instance from registry
├── reset.go        # DELETE — replaced by register --force
├── init.go         # MODIFY — stop generating preferred_port, stop showing port numbers
├── cmd_test.go     # MODIFY — update test commands from "up" to "register", add unregister tests
├── context.go      # NO CHANGE
├── root.go         # NO CHANGE
├── ports.go        # NO CHANGE
├── open.go         # NO CHANGE
├── status.go       # NO CHANGE
├── gc.go           # NO CHANGE
internal/
├── config/
│   ├── config.go      # NO CHANGE (preferred_port already optional at Go level)
│   └── config_test.go # MODIFY — add test for config without preferred_port
```

---

## Chunk 1: Rename `up` → `register` and remove `reset`

### Task 1: Rename up.go to register.go

**Files:**
- Rename: `cmd/up.go` → `cmd/register.go`
- Delete: `cmd/reset.go`

- [ ] **Step 1: Rename the file**

```bash
cd /Users/steve/src/outport
git mv cmd/up.go cmd/register.go
```

- [ ] **Step 2: Update the Cobra command in register.go**

In `cmd/register.go`, change the command definition:

```go
var registerCmd = &cobra.Command{
	Use:     "register",
	Aliases: []string{"reg"},
	Short:   "Register project and allocate ports",
	Long:    "Reads .outport.yml, allocates deterministic ports, saves to the central registry, and writes them to .env files.",
	RunE:    runRegister,
}
```

Update the `init()` function:

```go
func init() {
	registerCmd.Flags().BoolVar(&forceFlag, "force", false, "clear existing allocations and re-register all ports")
	rootCmd.AddCommand(registerCmd)
}
```

Rename `runUp` to `runRegister`:

```go
func runRegister(cmd *cobra.Command, args []string) error {
```

Update `printUpStyled` to `printRegisterStyled` and `printUpJSON` to `printRegisterJSON`. Update all references to these functions within the file. Update the `upJSON` type to `registerJSON`.

- [ ] **Step 3: Add hidden backward-compat alias for `up`**

Add to the bottom of `cmd/register.go`:

```go
var upCmd = &cobra.Command{
	Use:    "up",
	Hidden: true,
	Short:  "Alias for 'register' (deprecated)",
	RunE:   runRegister,
}

func init() {
	// This second init registers the hidden up command
}
```

Actually, Go doesn't allow two `init()` functions cleanly for this. Instead, add `upCmd` registration in the existing `init()`:

```go
func init() {
	registerCmd.Flags().BoolVar(&forceFlag, "force", false, "clear existing allocations and re-register all ports")
	rootCmd.AddCommand(registerCmd)

	// Hidden backward-compat alias
	upCmd := &cobra.Command{
		Use:    "up",
		Hidden: true,
		Short:  "Alias for 'register' (deprecated)",
		RunE:   runRegister,
	}
	upCmd.Flags().BoolVar(&forceFlag, "force", false, "")
	rootCmd.AddCommand(upCmd)
}
```

- [ ] **Step 4: Delete reset.go**

```bash
rm cmd/reset.go
```

The `register --force` flag replaces `reset` entirely.

- [ ] **Step 5: Verify it compiles**

```bash
go build -o dist/outport .
```

Expected: Compiles successfully.

- [ ] **Step 6: Commit**

```bash
git add cmd/register.go cmd/reset.go
git commit -m "feat: rename 'up' to 'register', remove 'reset' (use --force)"
```

---

### Task 2: Update tests for rename

**Files:**
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Update all "up" test commands to "register"**

Find and replace in test commands — every `executeCmd(t, "up"` becomes `executeCmd(t, "register"`. Every `executeCmd(t, "up", "--json")` becomes `executeCmd(t, "register", "--json")`. Every `executeCmd(t, "up", "--force"` becomes `executeCmd(t, "register", "--force"`.

Update test function names:
- `TestUp_AllocatesPortsAndWritesEnv` → `TestRegister_AllocatesPortsAndWritesEnv`
- `TestUp_IsIdempotent` → `TestRegister_IsIdempotent`
- `TestUp_StyledOutput` → `TestRegister_StyledOutput`
- `TestUp_NoConfig` → `TestRegister_NoConfig`
- `TestUp_ForceFlag` → `TestRegister_ForceFlag`
- `TestUp_GroupedConfig` → `TestRegister_GroupedConfig`
- `TestUp_GroupedStyledOutput` → `TestRegister_GroupedStyledOutput`

Update the `upJSON` references in tests to `registerJSON`.

Update `TestUp_NoConfig` to use `"register"`:

```go
rootCmd.SetArgs([]string{"register"})
```

Update `TestReset_ReallocatesWithPreferredPorts` to use `"register", "--force"`:

```go
func TestRegister_ForceReallocatesWithPreferredPorts(t *testing.T) {
	setupProject(t, testConfig)

	out1 := executeCmd(t, "register", "--json")
	var r1 registerJSON
	json.Unmarshal([]byte(out1), &r1)

	if r1.Services["web"].Port != 3000 {
		t.Errorf("first register: web port = %d, want 3000", r1.Services["web"].Port)
	}

	out2 := executeCmd(t, "register", "--force", "--json")
	var r2 registerJSON
	json.Unmarshal([]byte(out2), &r2)

	if r2.Services["web"].Port != 3000 {
		t.Errorf("force register: web port = %d, want 3000", r2.Services["web"].Port)
	}
	if r2.Services["postgres"].Port != 5432 {
		t.Errorf("force register: postgres port = %d, want 5432", r2.Services["postgres"].Port)
	}
}
```

Update the styled output test to check for "Ports written to" (which is the register output message, unchanged).

- [ ] **Step 2: Update tests that populate the registry via "up" for other commands**

Tests for `ports`, `status`, `open` use `executeCmd(t, "up", "--json")` to set up registry state. Change these to `executeCmd(t, "register", "--json")`.

- [ ] **Step 3: Run tests**

```bash
go test ./cmd/ -v
```

Expected: All tests PASS.

- [ ] **Step 4: Run full test suite**

```bash
just test
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: update tests for register rename"
```

---

## Chunk 2: Add `unregister` command

### Task 3: Add unregister command

**Files:**
- Create: `cmd/unregister.go`

- [ ] **Step 1: Write unregister test**

Add to `cmd/cmd_test.go`:

```go
// --- unregister ---

func TestUnregister_RemovesFromRegistry(t *testing.T) {
	setupProject(t, testConfig)

	// Register first
	executeCmd(t, "register", "--json")

	// Unregister
	output := executeCmd(t, "unregister")

	if !bytes.Contains([]byte(output), []byte("Unregistered")) {
		t.Errorf("expected 'Unregistered' message, got:\n%s", output)
	}

	// Verify ports command shows no allocation
	portsOutput := executeCmd(t, "ports")
	if !bytes.Contains([]byte(portsOutput), []byte("No ports allocated")) {
		t.Errorf("expected no ports after unregister, got:\n%s", portsOutput)
	}
}

func TestUnregister_NotRegistered(t *testing.T) {
	setupProject(t, testConfig)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"unregister"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func TestUnregister_JSON(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "register", "--json")

	output := executeCmd(t, "unregister", "--json")

	var result struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Project != "testapp" {
		t.Errorf("project = %q, want %q", result.Project, "testapp")
	}
	if result.Status != "unregistered" {
		t.Errorf("status = %q, want %q", result.Status, "unregistered")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/ -run TestUnregister -v
```

Expected: Compilation error — `unregister` command doesn't exist yet.

- [ ] **Step 3: Create cmd/unregister.go**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Remove project from the registry and free its ports",
	Long:  "Removes the current project/instance from the central registry, freeing all its port allocations.",
	RunE:  runUnregister,
}

func init() {
	rootCmd.AddCommand(unregisterCmd)
}

func runUnregister(cmd *cobra.Command, args []string) error {
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

	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	_, ok := reg.Get(cfg.Name, wt.Instance)
	if !ok {
		return fmt.Errorf("Project %q (instance %q) is not registered.", cfg.Name, wt.Instance)
	}

	reg.Remove(cfg.Name, wt.Instance)
	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printUnregisterJSON(cmd, cfg, wt)
	}
	return printUnregisterStyled(cmd, cfg, wt)
}

func printUnregisterJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info) error {
	out := struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Status   string `json:"status"`
	}{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Status:   "unregistered",
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printUnregisterStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, wt)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Unregistered. All ports freed."))
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/ -run TestUnregister -v
```

Expected: All three tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
just test
```

Expected: All tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/unregister.go cmd/cmd_test.go
git commit -m "feat: add 'unregister' command to free ports from registry"
```

---

## Chunk 3: Make `preferred_port` optional in init

### Task 4: Update outport init

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Remove port numbers from init presets and output**

Update the `servicePreset` struct to remove `PreferredPort`:

```go
type servicePreset struct {
	Name     string
	EnvVar   string
	Protocol string
}

var presets = []servicePreset{
	{"web", "PORT", "http"},
	{"postgres", "DATABASE_PORT", ""},
	{"redis", "REDIS_PORT", ""},
	{"mailpit_web", "MAILPIT_WEB_PORT", "http"},
	{"mailpit_smtp", "MAILPIT_SMTP_PORT", ""},
	{"vite", "VITE_PORT", "http"},
}
```

- [ ] **Step 2: Update the multi-select label**

Change the label from `fmt.Sprintf("%s (port %d)", p.Name, p.PreferredPort)` to just `p.Name`:

```go
for _, p := range presets {
	options = append(options, huh.NewOption(p.Name, p.Name))
}
```

- [ ] **Step 3: Update the YAML generation**

Remove the `preferred_port` line:

```go
for _, svc := range selectedServices {
	sb.WriteString(fmt.Sprintf("  %s:\n", svc.Name))
	sb.WriteString(fmt.Sprintf("    env_var: %s\n", svc.EnvVar))
	if svc.Protocol != "" {
		sb.WriteString(fmt.Sprintf("    protocol: %s\n", svc.Protocol))
	}
}
```

- [ ] **Step 4: Update the init output message**

Change the final message from `"Run 'outport up' to allocate ports."` to `"Run 'outport register' to allocate ports."`:

```go
fmt.Fprintln(cmd.OutOrStdout(), "Run 'outport register' to allocate ports.")
```

- [ ] **Step 5: Verify it compiles**

```bash
go build -o dist/outport .
```

Expected: Compiles successfully.

- [ ] **Step 6: Commit**

```bash
git add cmd/init.go
git commit -m "feat: init no longer asks about port numbers"
```

---

### Task 5: Add config test for no preferred_port

**Files:**
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Add test for config without preferred_port**

```go
func TestLoad_NoPreferredPort(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    protocol: http
  postgres:
    env_var: DATABASE_PORT
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(cfg.Services))
	}
	// PreferredPort should be 0 (Go default)
	if cfg.Services["web"].PreferredPort != 0 {
		t.Errorf("web.PreferredPort = %d, want 0", cfg.Services["web"].PreferredPort)
	}
	if cfg.Services["web"].EnvVar != "PORT" {
		t.Errorf("web.EnvVar = %q, want %q", cfg.Services["web"].EnvVar, "PORT")
	}
	if cfg.Services["web"].Protocol != "http" {
		t.Errorf("web.Protocol = %q, want %q", cfg.Services["web"].Protocol, "http")
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./internal/config/ -run TestLoad_NoPreferredPort -v
```

Expected: PASS. The config parser already treats missing `preferred_port` as 0, and the allocator already handles `preferred=0` as "use hash." This test just documents that behavior.

- [ ] **Step 3: Run full test suite**

```bash
just test
```

Expected: All tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: document that preferred_port is optional in config"
```

---

### Task 6: Update the outport skill and README

**Files:**
- Modify: `skills/outport/SKILL.md`

- [ ] **Step 1: Update the skill quick reference**

In `skills/outport/SKILL.md`, replace `outport up` references with `outport register`:

```markdown
## Quick Reference

```bash
outport init          # Create .outport.yml (interactive)
outport register      # Register project, allocate ports, write .env
outport reg           # Short alias for register
outport ports         # Show ports for current project
outport ports --json  # Machine-readable output
outport open          # Open HTTP services in browser
outport open web      # Open a specific service
outport status        # Show all registered projects
outport status --check # Show with health checks (up/down)
outport register --force  # Clear and re-allocate (tries preferred ports)
outport unregister    # Remove from registry, free ports
outport gc            # Remove stale registry entries
```
```

- [ ] **Step 2: Update config examples in SKILL.md**

Remove `preferred_port` from the primary config example. Keep it in an "advanced" section showing it's optional:

```yaml
# .outport.yml
name: my-project
services:
  web:
    env_var: PORT
    protocol: http
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
  mailpit_web:
    env_var: MAILPIT_WEB_PORT
    protocol: http
  mailpit_smtp:
    env_var: MAILPIT_SMTP_PORT
```

Add after the main example:

```markdown
### Optional: preferred_port

If you want Outport to try a specific port first (falling back to hash if taken), add `preferred_port`:

```yaml
services:
  web:
    env_var: PORT
    protocol: http
    preferred_port: 3000    # try this first, hash if taken
```
```

- [ ] **Step 3: Update "Run `outport up`" references in SKILL.md**

Replace "Run `outport up`" and "`outport up`" with "`outport register`" throughout the file.

- [ ] **Step 4: Commit**

```bash
git add skills/outport/SKILL.md
git commit -m "docs: update skill for register rename and optional preferred_port"
```

---

### Task 7: Lint and final verification

- [ ] **Step 1: Run linter**

```bash
just lint
```

Expected: No issues.

- [ ] **Step 2: Run full test suite**

```bash
just test
```

Expected: All tests PASS.

- [ ] **Step 3: Manual smoke test**

```bash
just build
dist/outport --help
dist/outport register --help
dist/outport unregister --help
dist/outport up --help    # should work (hidden alias)
```

Expected: All commands show correct help text. `up` is hidden but functional.

---

## Summary

| Change | What | Why |
|--------|------|-----|
| `up` → `register` | Rename command, add `reg` alias | Name says what it does — it registers, not "brings up" |
| Hidden `up` alias | Backward compat | Existing scripts/muscle-memory still work |
| Remove `reset` | Replaced by `register --force` | Less commands, same functionality |
| Add `unregister` | New command | Explicit way to free ports (replaces #12 concept) |
| `preferred_port` optional in init | Stop generating in new configs | Default experience is zero port thinking |
| Config test | Document optional behavior | Proves configs without `preferred_port` work |

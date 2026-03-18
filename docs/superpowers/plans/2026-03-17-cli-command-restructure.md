# CLI Command Restructure Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure outport's CLI commands into project-scoped top-level commands (`up`/`down`) and system-scoped subcommands (`system start`/`stop`/`restart`/`status`/`gc`/`uninstall`), eliminating the confusing flat command list.

**Architecture:** Cobra parent command `system` with six subcommands. Top-level `apply`→`up`, `unapply`→`down`. Current `setup`+`up` merged into `system start` with auto-setup. Current `teardown`→`system uninstall`. Grouped help output via Cobra `AddGroup()`.

**Tech Stack:** Go, Cobra CLI, existing `internal/` packages (unchanged)

**Spec:** `docs/superpowers/specs/2026-03-17-cli-command-restructure-design.md`

**Important ordering note:** `cmd/updown.go` defines `var upCmd`, `var downCmd`, `func runUp`, and `func runDown` at package scope. These names collide with the new project commands. Therefore, `updown.go` MUST be deleted (Task 2) BEFORE renaming `apply`→`up` (Task 3) and `unapply`→`down` (Task 4). The system subcommands that replace `updown.go` are created in the same task.

---

### Task 1: Create the `system` parent command and command groups

**Files:**
- Create: `cmd/system.go`
- Modify: `cmd/root.go`
- Modify: `cmd/cmdutil_test.go`
- Modify: `cmd/init.go`, `cmd/apply.go`, `cmd/ports.go`, `cmd/open.go`, `cmd/rename.go`, `cmd/promote.go` (add GroupID)

- [ ] **Step 1: Write the test for `system` subcommand discovery**

Add to `cmd/cmdutil_test.go`:

```go
func TestSystemCommandHasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"system"})
	if err != nil {
		t.Fatalf("system command not found: %v", err)
	}
	if !cmd.HasSubCommands() {
		t.Error("system command should have subcommands")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestSystemCommandHasSubcommands -v`
Expected: FAIL with "system command not found"

- [ ] **Step 3: Create `cmd/system.go` with parent command**

```go
package cmd

import "github.com/spf13/cobra"

var systemCmd = &cobra.Command{
	Use:     "system",
	Short:   "Manage the outport system (daemon, DNS, certificates)",
	Long:    "Commands for managing the machine-wide outport installation: daemon lifecycle, DNS resolver, CA certificates, and the global project registry.",
	GroupID: "system",
}

func init() {
	rootCmd.AddCommand(systemCmd)
}
```

- [ ] **Step 4: Add command groups to `cmd/root.go`**

In the `init()` function in `cmd/root.go`, add after the existing content:

```go
rootCmd.AddGroup(
	&cobra.Group{ID: "project", Title: "Project Commands:"},
	&cobra.Group{ID: "system", Title: "System Commands:"},
)
```

- [ ] **Step 5: Set GroupID on all existing project commands**

In each of these files, add `GroupID: "project"` to the cobra.Command struct:
- `cmd/init.go` — `initCmd`
- `cmd/apply.go` — `applyCmd`
- `cmd/ports.go` — `portsCmd`
- `cmd/open.go` — `openCmd`
- `cmd/rename.go` — `renameCmd`
- `cmd/promote.go` — `promoteCmd`

Do NOT set GroupID on `unapply.go`, `updown.go`, `setup.go`, `status.go`, or `gc.go` — those files will be restructured in later tasks.

- [ ] **Step 6: Update `TestAllCommandsHaveArgsValidation` to handle subcommands**

In `cmd/cmdutil_test.go`, update the test to skip the `system` parent command and recurse into subcommands:

```go
func TestAllCommandsHaveArgsValidation(t *testing.T) {
	skip := map[string]bool{
		"daemon":     true,
		"help":       true,
		"completion": true,
		"system":     true,
	}

	var allCmds []*cobra.Command
	for _, cmd := range rootCmd.Commands() {
		allCmds = append(allCmds, cmd)
		for _, sub := range cmd.Commands() {
			allCmds = append(allCmds, sub)
		}
	}
	for _, cmd := range allCmds {
		if skip[cmd.Name()] {
			continue
		}
		if cmd.Args == nil {
			t.Errorf("command %q has no Args validator — add NoArgs, ExactArgs, or MaximumArgs from cmdutil.go", cmd.Name())
		}
	}
}
```

- [ ] **Step 7: Run all tests**

Run: `just test`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add cmd/system.go cmd/root.go cmd/init.go cmd/apply.go cmd/ports.go cmd/open.go cmd/rename.go cmd/promote.go cmd/cmdutil_test.go
git commit -m "feat: add system parent command with grouped help output"
```

---

### Task 2: Delete `updown.go` and create system start/stop/restart/uninstall

This MUST happen before renaming `apply`→`up` and `unapply`→`down` because `updown.go` already defines `var upCmd`, `var downCmd`, `func runUp`, and `func runDown` at package scope.

**Files:**
- Delete: `cmd/updown.go`
- Rename: `cmd/setup.go` → `cmd/system_start.go`
- Create: `cmd/system_stop.go`
- Modify: `cmd/cmdutil_test.go`

- [ ] **Step 1: Write test for system subcommands**

Add to `cmd/cmdutil_test.go`:

```go
func TestSystemSubcommandsRejectArguments(t *testing.T) {
	subCmds := []string{"start", "stop", "restart", "uninstall"}

	for _, name := range subCmds {
		cmd, _, err := rootCmd.Find([]string{"system", name})
		if err != nil {
			t.Errorf("command system %q not found: %v", name, err)
			continue
		}

		validateErr := cmd.Args(cmd, []string{"unexpected-arg"})
		if validateErr == nil {
			t.Errorf("command system %q accepted unexpected arguments", name)
			continue
		}
		if !IsFlagError(validateErr) {
			t.Errorf("command system %q returned a plain error instead of FlagError: %v", name, validateErr)
		}
	}
}
```

Note: uses `t.Errorf` + `continue` (not `t.Fatalf`) so it reports all failures. The `status` and `gc` subcommands will be added to this list in Tasks 5-6.

- [ ] **Step 2: Remove old commands from `TestNoArgsCommandsRejectArguments`**

Update the `noArgsCmds` list to remove commands that are being restructured. Keep only what will still be top-level after all tasks:

```go
noArgsCmds := []string{
	"apply", "init", "ports", "promote", "unapply",
}
```

Note: `apply` and `unapply` are still the current names — they'll be updated in Tasks 3-4. `setup`, `teardown`, `up`, `down`, `status`, `gc` are removed since they're moving.

- [ ] **Step 3: Run test to verify the new test fails**

Run: `go test ./cmd/ -run TestSystemSubcommandsRejectArguments -v`
Expected: FAIL — subcommands don't exist yet

- [ ] **Step 4: Delete `cmd/updown.go`**

```bash
git rm cmd/updown.go
```

- [ ] **Step 5: Refactor `cmd/setup.go` into `cmd/system_start.go`**

```bash
git mv cmd/setup.go cmd/system_start.go
```

In `cmd/system_start.go`:

Replace `setupCmd` with `systemStartCmd`:
```go
var systemStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the outport system",
	Long:  "Starts the outport daemon. On first run, installs the .test DNS resolver, generates a local Certificate Authority, and adds it to the system trust store.",
	Args:  NoArgs,
	RunE:  runSystemStart,
}
```

Replace `teardownCmd` with `systemUninstallCmd`:
```go
var systemUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove outport system components",
	Long:  "Unloads the daemon, removes the LaunchAgent, DNS resolver, CA certificate, and cached server certs.",
	Args:  NoArgs,
	RunE:  runSystemUninstall,
}
```

Update `init()` to register with `systemCmd`:
```go
func init() {
	systemCmd.AddCommand(systemStartCmd)
	systemCmd.AddCommand(systemUninstallCmd)
}
```

Rename `runSetup` → `runSystemStart`. Update the logic to handle three cases:
1. Already set up AND agent loaded: `if jsonFlag { print {"status":"already_running"} } else { print "Outport system is already running." }; return`
2. Already set up but NOT loaded: just load the agent, print success
3. Not set up: run the full setup flow (existing code)

Remove the old early return that said `"Already set up. Use 'outport teardown' to remove and re-install."`

Rename `runTeardown` → `runSystemUninstall`. Update success message to `"Uninstall complete. DNS resolver, daemon, and certificates removed."`.

Rename JSON types: `setupJSON` → `systemStartJSON`, `teardownJSON` → `systemUninstallJSON`. Update print functions accordingly.

- [ ] **Step 6: Create `cmd/system_stop.go`**

```go
package cmd

import (
	"fmt"
	"os/exec"

	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var systemStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the outport system",
	Long:  "Unloads the LaunchAgent to stop the DNS resolver and HTTP proxy daemon.",
	Args:  NoArgs,
	RunE:  runSystemStop,
}

var systemRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the outport system",
	Long:  "Re-writes the LaunchAgent configuration and restarts the daemon. Use after upgrading outport.",
	Args:  NoArgs,
	RunE:  runSystemRestart,
}

func init() {
	systemCmd.AddCommand(systemStopCmd)
	systemCmd.AddCommand(systemRestartCmd)
}

func runSystemStop(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if !platform.IsSetup() {
		return fmt.Errorf("outport is not set up. Run 'outport system start' first")
	}

	if !platform.IsAgentLoaded() {
		if jsonFlag {
			fmt.Fprintln(w, `{"status": "already_stopped"}`)
			return nil
		}
		fmt.Fprintln(w, "Outport system is not running.")
		return nil
	}

	if err := platform.UnloadAgent(); err != nil {
		return err
	}

	if jsonFlag {
		fmt.Fprintln(w, `{"status": "stopped"}`)
		return nil
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system stopped."))
	return nil
}

func runSystemRestart(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if !platform.IsSetup() {
		return fmt.Errorf("outport is not set up. Run 'outport system start' to install")
	}

	// Re-write plist to pick up new binary path after upgrades
	outportBin, err := exec.LookPath("outport")
	if err != nil {
		return fmt.Errorf("could not find outport binary in PATH: %w", err)
	}
	if err := platform.WritePlist(outportBin); err != nil {
		return err
	}

	// Stop if running
	if platform.IsAgentLoaded() {
		if err := platform.UnloadAgent(); err != nil {
			return err
		}
	}

	// Start
	if err := platform.LoadAgent(); err != nil {
		return err
	}

	if jsonFlag {
		fmt.Fprintln(w, `{"status": "restarted"}`)
		return nil
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system restarted."))
	return nil
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./cmd/ -run "TestSystemSubcommandsRejectArguments|TestNoArgsCommandsRejectArguments|TestAllCommandsHaveArgsValidation" -v`
Expected: `TestSystemSubcommandsRejectArguments` PASS for start/stop/restart/uninstall. Other tests PASS.

- [ ] **Step 8: Run all tests**

Run: `just test`
Expected: All pass. Note: `cmd/cmd_test.go` tests referencing `setup`, `teardown`, `up` (daemon), `down` (daemon) may fail — these are addressed in Task 7.

- [ ] **Step 9: Commit**

```bash
git add cmd/system_start.go cmd/system_stop.go cmd/cmdutil_test.go
git rm cmd/updown.go
git commit -m "feat: add system start/stop/restart/uninstall, delete updown.go"
```

---

### Task 3: Rename `apply` → `up`

Now safe — `upCmd`/`runUp` names are gone since `updown.go` was deleted in Task 2.

**Files:**
- Rename: `cmd/apply.go` → `cmd/up.go`
- Modify: `cmd/ports.go` (uses `applyJSON` type)

- [ ] **Step 1: Rename the file**

```bash
git mv cmd/apply.go cmd/up.go
```

- [ ] **Step 2: Update the command definition in `cmd/up.go`**

Update the cobra.Command struct:
- `Use: "up"` (was `"apply"`)
- Remove `Aliases: []string{"a"}`
- `Short: "Bring this project into outport"` (was `"Apply port configuration and write .env files"`)
- `Long: "Registers this project, allocates deterministic ports, saves to the central registry, and writes them to .env files."`

Rename all symbols (find-and-replace across the file):
- `applyCmd` → `upCmd`
- `runApply` → `runUp`
- `printApplyJSON` → `printUpJSON`
- `printApplyStyled` → `printUpStyled`
- `applyJSON` → `upJSON`

Update `init()` to register `upCmd` with `rootCmd`.

- [ ] **Step 3: Update `cmd/ports.go` to use renamed type**

In `cmd/ports.go` line 61, change `applyJSON{` to `upJSON{`. Also update any other references to `applyJSON` in this file. Search the whole file for `applyJSON` and replace with `upJSON`.

- [ ] **Step 4: Add daemon hint to `runUp`**

At the end of `runUp`, before the final return, add:

```go
if !platform.IsAgentLoaded() {
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.DimStyle.Render("Hint: The outport daemon is not running. Run 'outport system start' to enable .test domains."))
}
```

Add `"github.com/outport-app/outport/internal/platform"` to imports. Check if `ui.DimStyle` exists — if not, use `ui.SubtleStyle` or add a Lipgloss dim style to `internal/ui/`.

- [ ] **Step 5: Update `TestNoArgsCommandsRejectArguments`**

In `cmd/cmdutil_test.go`, replace `"apply"` with `"up"` in the `noArgsCmds` list:

```go
noArgsCmds := []string{
	"up", "init", "ports", "promote", "unapply",
}
```

- [ ] **Step 6: Run all tests**

Run: `just test`
Expected: All pass except possibly `cmd/cmd_test.go` tests that call `executeCmd(t, "apply", ...)` — those are addressed in Task 7.

- [ ] **Step 7: Commit**

```bash
git add cmd/up.go cmd/ports.go cmd/cmdutil_test.go
git commit -m "feat: rename apply to up with daemon hint"
```

---

### Task 4: Rename `unapply` → `down`

Now safe — `downCmd`/`runDown` names are gone since `updown.go` was deleted in Task 2.

**Files:**
- Rename: `cmd/unapply.go` → `cmd/down.go`
- Modify: `cmd/cmdutil_test.go`

- [ ] **Step 1: Rename the file**

```bash
git mv cmd/unapply.go cmd/down.go
```

- [ ] **Step 2: Update the command definition in `cmd/down.go`**

Update the cobra.Command struct:
- `Use: "down"`
- `Short: "Remove this project from outport"` (was `"Remove ports and clean .env files"`)
- `Long: "Removes the managed block from all .env files and removes the project from the central registry."` (unchanged)
- Add `GroupID: "project"`

Rename all symbols (find-and-replace across the file):
- `unapplyCmd` → `downCmd`
- `runUnapply` → `runDown`
- `printUnapplyJSON` → `printDownJSON`
- `printUnapplyStyled` → `printDownStyled`

Update JSON output: change `"status": "unapplied"` to `"status": "removed"`.
Update styled output: change `"Unapplied. All ports freed."` to `"Done. All ports freed."`.

Update `init()` to register `downCmd` with `rootCmd`.

- [ ] **Step 3: Update `TestNoArgsCommandsRejectArguments`**

In `cmd/cmdutil_test.go`, replace `"unapply"` with `"down"`:

```go
noArgsCmds := []string{
	"up", "down", "init", "ports", "promote",
}
```

- [ ] **Step 4: Run all tests**

Run: `just test`
Expected: All pass except possibly `cmd/cmd_test.go` tests — addressed in Task 7.

- [ ] **Step 5: Commit**

```bash
git add cmd/down.go cmd/cmdutil_test.go
git commit -m "feat: rename unapply to down"
```

---

### Task 5: Move `status` under `system`

**Files:**
- Modify: `cmd/status.go`

- [ ] **Step 1: Move `status` to a system subcommand**

Rename `statusCmd` → `systemStatusCmd`.

Update `Short: "Show all registered projects"`.

Update `init()` to register with `systemCmd`:
```go
func init() {
	systemStatusCmd.Flags().BoolVar(&statusCheckFlag, "check", false, "check if ports are accepting connections")
	systemStatusCmd.Flags().BoolVar(&statusComputedFlag, "computed", false, "show computed values")
	systemCmd.AddCommand(systemStatusCmd)
}
```

- [ ] **Step 2: Remove the interactive stale-entry prompt**

In `printStatusStyled()`, find the `huh.Confirm` section (around line 241-268). Replace it with a non-interactive stale marker:

```go
if stale {
	fmt.Fprintf(w, "  %s\n", ui.DimStyle.Render("(stale — run 'outport system gc' to remove)"))
}
```

Remove the `huh` import if no longer used in this file.

- [ ] **Step 3: Update the "no projects" message**

Change from `"No projects registered. Run 'outport apply' in a project directory."` to `"No projects registered. Run 'outport up' in a project directory."`.

- [ ] **Step 4: Add `status` to the system subcommands test**

In `cmd/cmdutil_test.go`, update `TestSystemSubcommandsRejectArguments` to include `"status"`:

```go
subCmds := []string{"start", "stop", "restart", "status", "uninstall"}
```

- [ ] **Step 5: Run all tests**

Run: `just test`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add cmd/status.go cmd/cmdutil_test.go
git commit -m "feat: move status under system namespace"
```

---

### Task 6: Move `gc` under `system`

**Files:**
- Modify: `cmd/gc.go`
- Modify: `cmd/cmdutil_test.go`

- [ ] **Step 1: Move `gc` to a system subcommand**

Rename `gcCmd` → `systemGCCmd`.

Update `init()` to register with `systemCmd`:
```go
func init() {
	systemCmd.AddCommand(systemGCCmd)
}
```

- [ ] **Step 2: Add `gc` to the system subcommands test**

In `cmd/cmdutil_test.go`, update `TestSystemSubcommandsRejectArguments`:

```go
subCmds := []string{"start", "stop", "restart", "status", "gc", "uninstall"}
```

- [ ] **Step 3: Run all tests**

Run: `just test`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add cmd/gc.go cmd/cmdutil_test.go
git commit -m "feat: move gc under system namespace"
```

---

### Task 7: Update `cmd/cmd_test.go` for new command names

This is the main integration test file with 50+ tests. Every `executeCmd(t, "apply", ...)` must become `executeCmd(t, "up", ...)`, etc.

**Files:**
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Find and replace command names in test calls**

Search `cmd/cmd_test.go` for all references to old command names and replace:
- `"apply"` → `"up"` (in `executeCmd` calls)
- `"unapply"` → `"down"` (in `executeCmd` calls)
- `"status"` → `"system", "status"` (in `executeCmd` calls — note: this becomes two args)
- `"gc"` → `"system", "gc"` (in `executeCmd` calls — same)

Be careful: only replace command name arguments, not assertion strings that happen to contain these words in other contexts.

- [ ] **Step 2: Update assertion strings**

Search for old output strings and update:
- `"unapplied"` → `"removed"` (JSON status field)
- `"Unapplied"` → `"Done"` (styled output)
- Any references to `"outport apply"` or `"outport setup"` in expected output

- [ ] **Step 3: Rename test functions for clarity**

Rename test functions to match new command names:
- `TestApply_*` → `TestUp_*`
- `TestUnapply_*` → `TestDown_*`
- `TestStatus_*` → `TestSystemStatus_*`
- `TestGC_*` → `TestSystemGC_*`

This is optional but improves readability.

- [ ] **Step 4: Run all tests**

Run: `just test`
Expected: All pass

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: update integration tests for restructured commands"
```

---

### Task 8: Update user-facing strings in project commands and internals

**Files:**
- Modify: `cmd/init.go`
- Modify: `cmd/ports.go`
- Modify: `cmd/open.go`
- Modify: `internal/daemon/proxy.go`
- Modify: `internal/platform/other.go`

- [ ] **Step 1: Update `cmd/init.go`**

Three lines to change:
- Line 27 (in `configTemplate`): `"outport apply"` → `"outport up"`
- Line 29 (in `configTemplate`): `"outport setup"` → `"outport system start"`
- Line 85: `"Edit it for your project, then run 'outport apply' to allocate ports."` → `"Edit it for your project, then run 'outport up' to allocate ports."`

- [ ] **Step 2: Update `cmd/ports.go`**

Line 38: `"No ports allocated. Run 'outport apply' first."` → `"No ports allocated. Run 'outport up' first."`

- [ ] **Step 3: Update `cmd/open.go`**

Line 34: `"No ports allocated. Run 'outport apply' first."` → `"No ports allocated. Run 'outport up' first."`
Line 82: `"No port allocated for %q. Run 'outport apply' first."` → `"No port allocated for %q. Run 'outport up' first."`

- [ ] **Step 4: Update `internal/daemon/proxy.go`**

Line 49-50: Change `"outport apply"` → `"outport up"` in the HTML error page.

- [ ] **Step 5: Update `internal/platform/other.go`**

Line 7: Change `"outport setup is only supported on macOS"` → `"outport system start is only supported on macOS"`.

- [ ] **Step 6: Run all tests**

Run: `just test`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add cmd/init.go cmd/ports.go cmd/open.go internal/daemon/proxy.go internal/platform/other.go
git commit -m "fix: update user-facing strings to reference new command names"
```

---

### Task 9: Cleanup — lint, `go mod tidy`, verification

- [ ] **Step 1: Run `go mod tidy`**

The `huh` package may no longer be imported anywhere after removing the interactive prompt from `status.go`. Run:

```bash
go mod tidy
```

- [ ] **Step 2: Run linter**

Run: `just lint`
Expected: All pass. Fix any issues.

- [ ] **Step 3: Run full test suite**

Run: `just test`
Expected: All pass.

- [ ] **Step 4: Build and smoke test help output**

```bash
just build
./dist/outport --help
./dist/outport system --help
```

Verify help shows grouped "Project Commands:" and "System Commands:" sections as designed in the spec.

- [ ] **Step 5: Smoke test old commands are gone**

```bash
./dist/outport apply 2>&1    # should error "unknown command"
./dist/outport setup 2>&1    # should error "unknown command"
```

- [ ] **Step 6: Commit any fixes**

```bash
git add -A
git commit -m "chore: lint, go mod tidy, and cleanup"
```

---

### Task 10: Update documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/commands.md`
- Modify: `docs/guide/getting-started.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update `README.md`**

Replace all old command names:
- `outport apply` → `outport up`
- `outport unapply` → `outport down`
- `outport setup` → `outport system start`
- `outport teardown` → `outport system uninstall`
- `outport up` (daemon) → `outport system start`
- `outport down` (daemon) → `outport system stop`
- `outport status` → `outport system status`
- `outport gc` → `outport system gc`

Update the getting started section to reflect the new user journey.

- [ ] **Step 2: Update `docs/reference/commands.md`**

Restructure into "Project Commands" and "System Commands" sections. Replace all old command names and update all code examples.

- [ ] **Step 3: Update `docs/guide/getting-started.md`**

Update the getting started flow:
1. `brew install outport`
2. `outport system start`
3. `outport init` + edit
4. `outport up`

- [ ] **Step 4: Update `CLAUDE.md`**

Update the CLI commands section, design decisions, and finalize checklist. Key replacements:
- All command name references
- Architecture section: `system` subcommand structure
- Command descriptions in the "CLI commands" section

- [ ] **Step 5: Check for `README-VISION.md` or other docs**

Search for any other files referencing old command names:
```bash
grep -r "outport apply\|outport unapply\|outport setup\|outport teardown" --include="*.md" .
```

Update any found. Skip historical spec/plan docs in `docs/superpowers/`.

- [ ] **Step 6: Verify docs build**

Run: `cd docs && npm run docs:build`
Expected: Build succeeds.

- [ ] **Step 7: Commit**

```bash
git add README.md docs/ CLAUDE.md
git commit -m "docs: update all documentation for command restructure"
```

---

### Task 11: Final verification

- [ ] **Step 1: Run the full finalize checklist**

- `just lint` passes
- `just test` passes
- README.md commands list matches actual commands in `cmd/`
- `init` presets in `cmd/init.go` reference correct commands (`outport up`, `outport system start`)
- `--json` output works for changed commands: `outport system start --json`, `outport system stop --json`
- CLAUDE.md reflects the restructure
- Docs updated and build succeeds

- [ ] **Step 2: Manual smoke test of key workflows**

```bash
./dist/outport --help
./dist/outport system --help
./dist/outport up --help
./dist/outport down --help
./dist/outport system start --help
./dist/outport system stop --help
./dist/outport system restart --help
./dist/outport system status --help
./dist/outport system gc --help
./dist/outport system uninstall --help
```

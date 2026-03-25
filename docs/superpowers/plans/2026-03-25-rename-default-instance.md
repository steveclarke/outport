# Rename Default-to-Current-Instance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `outport rename <new>` (1 arg) to rename the current directory's instance, defaulting `oldName` to the resolved instance.

**Architecture:** Add a `RangeArgs` validator to `cmdutil.go`, update the rename command to accept 1–2 args, and derive `oldName` from `ctx.Instance` when only 1 arg is given. Follows the same implicit-instance pattern as `promote`.

**Tech Stack:** Go, Cobra CLI

**Spec:** `docs/superpowers/specs/2026-03-25-rename-default-instance-design.md`

---

### Task 1: Add `RangeArgs` validator

**Files:**
- Modify: `cmd/cmdutil.go:86-104` (add after `MaximumArgs`, before `MinimumArgs`)
- Test: `cmd/cmdutil_test.go`

- [ ] **Step 1: Write the failing test for `RangeArgs`**

Add to `cmd/cmdutil_test.go` after `TestMinimumArgsHelper`:

```go
func TestRangeArgsHelper(t *testing.T) {
	v := RangeArgs(1, 2, "requires 1 or 2 args")
	testArgsValidator(t, v, []string{"a"}, false, false)
	testArgsValidator(t, v, []string{"a", "b"}, false, false)
	testArgsValidator(t, v, []string{}, true, true)
	testArgsValidator(t, v, []string{"a", "b", "c"}, true, true)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestRangeArgsHelper -v`
Expected: FAIL — `RangeArgs` undefined.

- [ ] **Step 3: Implement `RangeArgs`**

Add to `cmd/cmdutil.go` after `MaximumArgs`:

```go
// RangeArgs returns a Cobra arg validator that accepts between min and max args (inclusive).
func RangeArgs(min, max int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > max {
			return FlagErrorf("too many arguments")
		}
		if len(args) < min {
			return FlagErrorf("%s", msg)
		}
		return nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestRangeArgsHelper -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/cmdutil.go cmd/cmdutil_test.go
git commit -m "feat: add RangeArgs validator to cmdutil"
```

---

### Task 2: Update rename command to accept 1 or 2 args

**Files:**
- Modify: `cmd/rename.go:15-22` (command definition), `cmd/rename.go:28-30` (arg handling)

- [ ] **Step 1: Update the command definition**

In `cmd/rename.go`, change the `Use` and `Args` fields:

```go
var renameCmd = &cobra.Command{
	Use:     "rename [old] <new>",
	Short:   "Rename an instance of the current project",
	Long:    "Renames an instance in the registry and updates hostnames in .env files.\nIf only one argument is given, renames the current directory's instance.",
	GroupID: "project",
	Args:    RangeArgs(1, 2, "requires at least one argument: outport rename <new-name>"),
	RunE:    runRename,
}
```

- [ ] **Step 2: Update arg handling in `runRename`**

Replace the first 3 lines of `runRename` (lines 29–30) with:

```go
func runRename(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	var oldName, newName string
	if len(args) == 2 {
		oldName = args[0]
		newName = args[1]
	} else {
		oldName = ctx.Instance
		newName = args[0]
	}
```

Note: `loadProjectContext()` moves above the arg parsing since the 1-arg path
needs `ctx.Instance`. Remove the duplicate `ctx, err := loadProjectContext()`
that was on line 40.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Run existing rename tests to confirm no regressions**

Run: `go test ./cmd/ -run TestRename -v`
Expected: All 3 existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/rename.go
git commit -m "feat: rename defaults to current instance with 1 arg"
```

---

### Task 3: Update arg validation tests

**Files:**
- Modify: `cmd/cmdutil_test.go:61-98` (`TestExactArgsCommands`)

- [ ] **Step 1: Update `TestExactArgsCommands`**

The `rename` entry currently expects 1 arg to be "tooFew". Move it to a new
`TestRangeArgsCommands` test and remove from `TestExactArgsCommands`:

Delete the entire `TestExactArgsCommands` function (lines 61–98) since `rename`
is its only entry.

Add a new test after `TestMaximumArgsCommands`:

```go
func TestRangeArgsCommands(t *testing.T) {
	tests := []struct {
		name    string
		valid   [][]string
		invalid [][]string
	}{
		{
			name:    "rename",
			valid:   [][]string{{"new"}, {"old", "new"}},
			invalid: [][]string{{}, {"a", "b", "c"}},
		},
	}

	for _, tt := range tests {
		cmd, _, err := rootCmd.Find([]string{tt.name})
		if err != nil {
			t.Fatalf("command %q not found: %v", tt.name, err)
		}

		for _, args := range tt.valid {
			if err := cmd.Args(cmd, args); err != nil {
				t.Errorf("%s: rejected valid args %v: %v", tt.name, args, err)
			}
		}

		for _, args := range tt.invalid {
			if err := cmd.Args(cmd, args); err == nil {
				t.Errorf("%s: accepted invalid args %v", tt.name, args)
			} else if !IsFlagError(err) {
				t.Errorf("%s: error for args %v is not FlagError: %v", tt.name, args, err)
			}
		}
	}
}
```

- [ ] **Step 2: Run updated tests**

Run: `go test ./cmd/ -run "TestExactArgs|TestRangeArgsCommands" -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/cmdutil_test.go
git commit -m "test: update arg validation tests for rename range args"
```

---

### Task 4: Add integration tests for 1-arg rename

**Files:**
- Modify: `cmd/cmd_test.go` (add after existing rename tests, ~line 967)

- [ ] **Step 1: Write test for 1-arg rename from main instance**

Add after `TestRename_Success`:

```go
func TestRename_OneArg_FromMain(t *testing.T) {
	setupProject(t, testConfigWithHostnames)
	executeCmd(t, "up", "--json")

	// Rename current instance (main) → staging using 1-arg form
	output := executeCmd(t, "rename", "--json", "staging")

	var result struct {
		OldInstance string `json:"old_instance"`
		NewInstance string `json:"new_instance"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.OldInstance != "main" {
		t.Errorf("old_instance = %q, want main", result.OldInstance)
	}
	if result.NewInstance != "staging" {
		t.Errorf("new_instance = %q, want staging", result.NewInstance)
	}
	if result.Status != "renamed" {
		t.Errorf("status = %q, want renamed", result.Status)
	}
}
```

- [ ] **Step 2: Run it to verify it passes**

Run: `go test ./cmd/ -run TestRename_OneArg_FromMain -v`
Expected: PASS

- [ ] **Step 3: Write test for 1-arg rename from non-main instance**

This test creates a second checkout directory to get a non-main instance, then
renames it using the 1-arg form.

Add after `TestRename_OneArg_FromMain`:

```go
func TestRename_OneArg_FromNonMain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	jsonFlag = false
	yesFlag = false
	forceFlag = false
	isPortBusy = func(int) bool { return false }

	// Create "main" instance in dir1
	dir1 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "outport.yml"), []byte(testConfigWithHostnames), 0644)
	os.Mkdir(filepath.Join(dir1, ".git"), 0755)
	t.Chdir(dir1)
	executeCmd(t, "up", "--json")

	// Create second instance in dir2 (will get an auto-generated code)
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "outport.yml"), []byte(testConfigWithHostnames), 0644)
	os.Mkdir(filepath.Join(dir2, ".git"), 0755)
	t.Chdir(dir2)
	out := executeCmd(t, "up", "--json")

	var upResult struct {
		Instance string `json:"instance"`
	}
	json.Unmarshal([]byte(out), &upResult)
	autoCode := upResult.Instance

	// Rename current (auto-code) → "dev" using 1-arg form
	output := executeCmd(t, "rename", "--json", "dev")

	var result struct {
		OldInstance string `json:"old_instance"`
		NewInstance string `json:"new_instance"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.OldInstance != autoCode {
		t.Errorf("old_instance = %q, want %q", result.OldInstance, autoCode)
	}
	if result.NewInstance != "dev" {
		t.Errorf("new_instance = %q, want dev", result.NewInstance)
	}
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./cmd/ -run TestRename_OneArg_FromNonMain -v`
Expected: PASS

- [ ] **Step 5: Run all tests**

Run: `just test`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test: add 1-arg rename integration tests"
```

---

### Task 5: Update docs site

**Files:**
- Modify: `docs/reference/commands.md:118-126` (rename command section)

- [ ] **Step 1: Update the rename command docs**

Change the rename section in `docs/reference/commands.md`:

```markdown
### `outport rename`

Rename an instance of the current project.

```bash
outport rename [old-name] <new-name>
```

If `old-name` is omitted, renames the current directory's instance. Updates the
instance name in the registry and regenerates hostnames in `.env` files.
```

- [ ] **Step 2: Commit**

```bash
git add docs/reference/commands.md
git commit -m "docs: update rename command syntax for optional old-name"
```

---

### Task 6: Lint and final verification

- [ ] **Step 1: Run linter**

Run: `just lint`
Expected: PASS

- [ ] **Step 2: Run full test suite**

Run: `just test`
Expected: All tests pass.

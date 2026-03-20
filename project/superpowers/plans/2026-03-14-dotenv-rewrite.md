# Dotenv Rewrite Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the dotenv merge logic to always overwrite variables declared in `outport.yml`, remove the marker system, and use `github.com/joho/godotenv` for proper `.env` parsing.

**Architecture:** Replace the hand-rolled string manipulation in `internal/dotenv/dotenv.go` with `godotenv` for reading `.env` files. Outport always overwrites variables it manages (as declared in the config). Variables NOT in the config are preserved untouched. No markers needed — the `outport.yml` is the source of truth for what Outport owns.

**Tech Stack:** Go 1.24+, `github.com/joho/godotenv` (https://github.com/joho/godotenv)

---

## File Structure

```
internal/
├── dotenv/
│   ├── dotenv.go       # REWRITE — use godotenv, remove markers, always overwrite managed vars
│   └── dotenv_test.go  # REWRITE — new test cases matching new behavior
```

No other files change. The `Merge(path, ports)` function signature stays the same so callers (`cmd/register.go`) don't need updating.

---

## Chunk 1: Rewrite dotenv package

### Task 1: Add godotenv dependency

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add the dependency**

```bash
cd /Users/steve/src/outport
go get github.com/joho/godotenv
```

- [ ] **Step 2: Verify it's in go.mod**

```bash
grep godotenv go.mod
```

Expected: `github.com/joho/godotenv v1.x.x`

---

### Task 2: Rewrite tests for new behavior

**Files:**
- Rewrite: `internal/dotenv/dotenv_test.go`

- [ ] **Step 1: Replace dotenv_test.go entirely**

The key behavioral changes:
- No more markers — Outport doesn't append `# managed by outport` to lines
- Variables that exist in the file AND are in the ports map get overwritten (this was the bug)
- Variables that exist in the file but are NOT in the ports map are preserved
- Comments and blank lines are preserved
- New variables are appended at the end
- Handles files with `export` prefix, quoted values, etc. (godotenv handles these)

```go
package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMerge_NewFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{
		"PORT":          "31653",
		"DATABASE_PORT": "17842",
	}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653")
	}
	if !strings.Contains(content, "DATABASE_PORT=17842") {
		t.Error("missing DATABASE_PORT=17842")
	}
}

func TestMerge_PreservesUnrelatedVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET_KEY=abc123\nRAILS_ENV=development\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
	if !strings.Contains(content, "RAILS_ENV=development") {
		t.Error("lost existing RAILS_ENV")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653")
	}
}

func TestMerge_OverwritesExistingVar(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// This is the critical test — Outport MUST overwrite existing values
	existing := "PORT=4000\nSECRET_KEY=abc123\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if strings.Contains(content, "PORT=4000") {
		t.Error("old PORT value should be overwritten")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing updated PORT=31653")
	}
	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
}

func TestMerge_UpdatesValueInPlace(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// Variable should be updated where it appears, not moved to the end
	existing := "FIRST=1\nPORT=4000\nLAST=3\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	// PORT should still be on line 2 (index 1), not appended
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[1] != "PORT=31653" {
		t.Errorf("line 2 = %q, want PORT=31653", lines[1])
	}
}

func TestMerge_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "# Database config\nDB_PORT=5432\n\n# Redis\nREDIS_PORT=6379\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"DB_PORT": "21536"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "# Database config") {
		t.Error("lost comment")
	}
	if !strings.Contains(content, "DB_PORT=21536") {
		t.Error("missing updated DB_PORT")
	}
	if !strings.Contains(content, "REDIS_PORT=6379") {
		t.Error("lost unrelated REDIS_PORT")
	}
}

func TestMerge_PreservesBlankLines(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "FIRST=1\n\nSECOND=2\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"FIRST": "10"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "\n\n") {
		t.Error("blank line was removed")
	}
}

func TestMerge_HandlesExportPrefix(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "export PORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if strings.Contains(content, "4000") {
		t.Error("old value should be overwritten")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing updated PORT")
	}
}

func TestMerge_HandlesQuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET=\"my secret\"\nPORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "SECRET=\"my secret\"") {
		t.Error("lost quoted SECRET value")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing updated PORT")
	}
}

func TestMerge_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{"PORT": "31653", "DB_PORT": "17842"}

	Merge(envPath, ports)
	data1, _ := os.ReadFile(envPath)

	Merge(envPath, ports)
	data2, _ := os.ReadFile(envPath)

	if string(data1) != string(data2) {
		t.Errorf("merge is not idempotent:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
}

func TestMerge_CommentedOutVarIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// A commented-out variable should stay commented; Outport appends a new active line
	existing := "# PORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "# PORT=4000") {
		t.Error("commented line should be preserved")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing appended PORT")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/dotenv/ -v
```

Expected: Several failures — the old implementation skips existing vars and adds markers.

---

### Task 3: Rewrite dotenv.go

**Files:**
- Rewrite: `internal/dotenv/dotenv.go`

- [ ] **Step 1: Replace dotenv.go entirely**

The new implementation:
1. Read the file as lines (preserve structure, comments, blank lines)
2. Parse variable names from each line (handle `export` prefix)
3. For any line whose variable name is in the ports map, replace the line with `VAR=value`
4. For ports not found in existing lines, append them at the end
5. Write the file back

We use `godotenv` for robust parsing when we need to read values, but for the merge we do line-by-line processing to preserve file structure (comments, blank lines, ordering). `godotenv.Read()` would lose all of that.

```go
package dotenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Merge writes port values into the .env file at path.
// Variables declared in ports are always overwritten if they exist.
// Variables not in ports are preserved untouched.
// Comments and blank lines are preserved.
func Merge(path string, ports map[string]string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .env: %w", err)
	}

	written := make(map[string]bool)

	// Update existing lines in place
	for i, line := range lines {
		name := parseVarName(line)
		if name == "" {
			continue
		}
		if value, ok := ports[name]; ok {
			lines[i] = fmt.Sprintf("%s=%s", name, value)
			written[name] = true
		}
	}

	// Append any new variables not already in the file
	var newVars []string
	for name := range ports {
		if !written[name] {
			newVars = append(newVars, name)
		}
	}
	sort.Strings(newVars)

	if len(newVars) > 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		for _, name := range newVars {
			lines = append(lines, fmt.Sprintf("%s=%s", name, ports[name]))
		}
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

// parseVarName extracts the variable name from a line.
// Handles "VAR=value" and "export VAR=value".
// Returns "" for comments, blank lines, and lines without =.
func parseVarName(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Strip export prefix
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimPrefix(trimmed, "export ")
		trimmed = strings.TrimSpace(trimmed)
	}

	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
```

Note: We're not importing `godotenv` in this implementation. The line-by-line approach with `parseVarName` handling `export` prefix is sufficient and preserves file structure. `godotenv.Read()` would flatten the file into a map, losing comments, blank lines, and ordering. If we need `godotenv` for more complex parsing in the future, we can add it then — YAGNI.

- [ ] **Step 2: Run tests**

```bash
go test ./internal/dotenv/ -v
```

Expected: All tests PASS.

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -v
```

Expected: All tests PASS. The `Merge` function signature didn't change, so callers are unaffected.

- [ ] **Step 4: Remove godotenv dependency if not used**

Since we ended up not importing godotenv:

```bash
go mod tidy
```

This will remove it from go.mod/go.sum if it's not imported anywhere.

- [ ] **Step 5: Commit**

```bash
git add internal/dotenv/ go.mod go.sum
git commit -m "feat: rewrite dotenv merge — always overwrite managed vars, remove markers"
```

---

## Summary

| Before | After |
|--------|-------|
| Existing vars without marker silently skipped | Existing vars always overwritten if declared in config |
| `# managed by outport` marker on every line | No markers — `outport.yml` is the source of truth |
| Hand-rolled parsing, no `export` handling | Handles `export` prefix, quoted values preserved |
| `TestMerge_DoesNotClobberUserVar` tested the bug | `TestMerge_OverwritesExistingVar` tests the fix |
| First run on existing `.env` appeared to work but didn't | First run actually works |

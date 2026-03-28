# Structured Error Hints

**Date:** 2026-03-26
**Status:** Design

## Problem

When a CLI command fails, terminal users see a bare error message with no guidance on what to do next. The `--json` output path already maps common errors to hints via `jsonErrorHint()` in `root.go`, but styled terminal output just prints the error to stderr and exits.

## Design

### Hint display

When a command fails, check the error message against a hint table. If there's a match, print the hint on the next line in dimmed style:

```
Error: No outport.yml found in /Users/steve/myproject or any parent directory.
Hint:  Run: outport init
```

### Hint table

The existing `jsonErrorHint()` function in `root.go` becomes a shared `errorHint()` function. Both JSON and styled output use it.

Starting set of hints:

| Error contains | Hint |
|---|---|
| `"No outport.yml"` | `Run: outport init` |
| `"not registered"` | `Run: outport up` |
| `"No ports allocated"` | `Run: outport up` |

These are the three hints already in `jsonErrorHint()`. New hints can be added over time as common failure patterns emerge.

### Where the change happens

**`cmd/root.go`:**
- Rename `jsonErrorHint()` to `errorHint()` (same logic, shared by both paths).
- In `Execute()`, the JSON path already calls this — no change needed there.

**`main.go`:**
- After printing the error to stderr, call `errorHint()`. If non-empty, print it on the next line using `ui.DimStyle`.

### What doesn't change

- `FlagError` still prints usage — no hint needed, the usage string is the hint.
- `ErrSilent` still exits quietly — no output at all.
- Doctor keeps its own `Fix` field per check result — separate system.
- Proxy error pages keep their own inline hints in HTML.

### Testing

- Table-driven test for `errorHint()`: each hint table entry gets a test case confirming the match, plus a "no match" case returning empty string.
- Integration: covered by existing BATS E2E tests (errors already tested for correct messages; hints are additive).

## Scope

This is a small change: rename one function, add a few lines to `main.go`, add one test file. The hint table grows organically as we see what users trip on.

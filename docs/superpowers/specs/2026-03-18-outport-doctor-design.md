# `outport doctor` — Diagnostic Command

**Date:** 2026-03-18
**Issue:** #35 (supersedes #9)

## Summary

A top-level `outport doctor` command that checks the health of all Outport infrastructure and reports pass/warn/fail for each check with actionable fix suggestions. Read-only — never modifies system state.

## Command

```
outport doctor [--json]
```

- Top-level command (not under `system`), since it checks both system and project health.
- Supports `--json` for machine-readable output.
- Exit code 0 if all checks pass or warn. Exit code 1 if any check fails.

## Architecture

### Package: `internal/doctor/`

Core types:

```go
type Status int

const (
    Pass Status = iota
    Warn
    Fail
)

type Check struct {
    Name     string
    Category string         // "System" or "Project (myapp)" — project name comes from config.Name
    Run      func() *Result
}

type Result struct {
    Name    string
    Status  Status
    Message string // human-readable description of what was checked
    Fix     string `json:"fix,omitempty"` // actionable suggestion on fail/warn, omitted on pass
}

type Runner struct {
    checks []Check
}
```

Runner methods:

- `Add(check Check)` — appends a check.
- `Run() []Result` — executes all checks sequentially, returns results.

Sequential execution because checks are fast (file stats, single TCP dials at ~200ms) and order matters for readable output. No dependency or short-circuiting between checks — every check runs independently so the user sees the full picture.

### Command: `cmd/doctor.go`

Registers checks and handles output:

1. Always registers system checks.
2. Gets the working directory via `os.Getwd()`, then attempts `config.FindDir(cwd)` (matching existing command patterns like `loadProjectContext()`):
   - If found, loads config once via `config.Load(dir)`. If config is valid, registers project checks using the loaded config and dir. If config is invalid, registers a single failing config check and skips remaining project checks.
   - If no `outport.yml` found, no project checks are added (no error).
3. Runs all checks via the Runner.
4. Outputs styled or JSON results.

**Implementation notes:**

- `platform.plistPath()` is currently unexported — export it as `platform.PlistPath()` so the doctor can stat and parse the plist file.
- The resolver content string (`"nameserver 127.0.0.1\nport 15353\n"`) should be extracted as `platform.ResolverContent` so the doctor check can't drift from `WriteResolverFile()`.
- CA trust verification needs a new `platform.IsCA Trusted(certPath string) bool` function wrapping `security verify-cert -c {certPath}`.
- Registry check #13 requires a two-step approach: first `os.Stat` the registry path (missing → Warn), then `registry.Load()` (parse error → Fail). This is because `registry.Load()` returns `(emptyRegistry, nil)` for missing files — it doesn't error.
- DNS resolution check #6 introduces new logic (UDP query via `miekg/dns`) not wrapped by an existing function. Extract this into a testable function that accepts a resolver address.

## Output Format

### Styled (default)

```
System
  ✓ DNS resolver file exists (/etc/resolver/test)
  ✓ DNS resolver content correct
  ✓ LaunchAgent plist installed
  ✓ LaunchAgent plist binary valid
  ✓ LaunchAgent loaded
  ✓ DNS resolving *.test → 127.0.0.1
  ✓ HTTP proxy responding (port 80)
  ✓ HTTPS proxy responding (port 443)
  ✓ CA certificate exists
  ✓ CA private key exists
  ✓ CA certificate not expired
  ✓ CA trusted in system keychain
  ✓ Registry file valid
  ✓ cloudflared installed

Project (myapp)
  ✓ outport.yml valid
  ✓ Project registered (myapp/main)
  ✓ Port 12345 (web) available
  ✓ Port 12346 (postgres) available

All checks passed.
```

Failures:

```
  ✗ LaunchAgent not loaded
    → Run: outport system start
```

Warnings:

```
  ! cloudflared not installed
    → Install with: brew install cloudflared
```

### Severity Levels

- **Pass** — check succeeded.
- **Warn** — not broken, but worth knowing. Does not affect exit code.
- **Fail** — something is broken. Causes exit code 1.

### JSON

```json
{
  "results": [
    {
      "name": "DNS resolver file exists",
      "category": "System",
      "status": "pass",
      "message": "DNS resolver file exists (/etc/resolver/test)"
    },
    {
      "name": "cloudflared installed",
      "category": "System",
      "status": "warn",
      "message": "cloudflared not installed",
      "fix": "Install with: brew install cloudflared"
    }
  ],
  "passed": true
}
```

`passed` is `false` if any result has status `"fail"`.

## Checks

### System Checks (always run)

| # | Check | Implementation | On Failure |
|---|-------|---------------|------------|
| 1 | DNS resolver file exists | `os.Stat("/etc/resolver/test")` | Fail → `outport system start` |
| 2 | DNS resolver content correct | `os.ReadFile`, compare to `"nameserver 127.0.0.1\nport 15353\n"` | Fail → `outport system start` |
| 3 | LaunchAgent plist installed | `os.Stat(plistPath())` | Fail → `outport system start` |
| 4 | LaunchAgent plist binary valid | Parse plist XML, extract `ProgramArguments[0]`, `os.Stat` the binary | Fail → `outport system restart` |
| 5 | LaunchAgent loaded | `platform.IsAgentLoaded()` | Fail → `outport system start` |
| 6 | DNS resolving | UDP query to `127.0.0.1:15353` for `anything.test`, expect A record `127.0.0.1` | Fail → `outport system restart` |
| 7 | HTTP proxy responding | `portcheck.IsUp(80)` | Fail → `outport system restart` |
| 8 | HTTPS proxy responding | `portcheck.IsUp(443)` | Fail → `outport system restart` |
| 9 | CA certificate exists | `os.Stat` on `~/.local/share/outport/ca-cert.pem` | Fail → `outport system start` |
| 10 | CA private key exists | `os.Stat` on `~/.local/share/outport/ca-key.pem` | Fail → `outport system start` |
| 11 | CA not expired | Parse x509 cert, check `NotAfter > now()` | Fail → `outport system uninstall && outport system start` |
| 12 | CA trusted in keychain | `security verify-cert -c {caCertPath}` | Fail → `outport system start` |
| 13 | Registry file valid | `registry.Load()` — missing file is Warn, parse error is Fail | Warn: no projects yet. Fail: corrupted file. |
| 14 | cloudflared installed | `exec.LookPath("cloudflared")` | **Warn** → `brew install cloudflared` |

### Project Checks (only when `outport.yml` found)

| # | Check | Implementation | On Failure |
|---|-------|---------------|------------|
| 15 | outport.yml valid | `config.Load(dir)` — reports the validation error directly | Fail |
| 16 | Project registered | `reg.FindByDir(dir)` | Fail → `outport up` |
| 17 | Port available (per service) | `portcheck.IsUp(port)` per allocated port | **Warn** — port in use is informational since the service may legitimately be running |

Port conflict nuance: a port being "in use" isn't necessarily bad — it could be the actual service running. So port checks are Warn not Fail, with a message like "Port 5432 (postgres) is in use".

If `config.Load()` fails (check 15), remaining project checks (16, 17) are skipped since they depend on a valid config.

## Testing

The `internal/doctor/` package is testable via check functions:

**Runner tests:**
- All-pass checks → `passed: true`, exit 0
- Runner with a warn → `passed: true`, exit 0
- Runner with a fail → `passed: false`, exit 1
- Mixed results → correct ordering, counts, and `passed` value

**Check-specific tests** for logic that isn't trivially delegated:
- Plist binary path parsing and validation
- Resolver content comparison
- CA cert expiry parsing

Individual checks are thin wrappers around existing tested functions (`portcheck.IsUp`, `config.Load`, `registry.Load`, `certmanager.IsCAInstalled`, `platform.IsAgentLoaded`), so they don't need deep unit tests.

## Not In Scope

- **`--fix` auto-repair** — `outport system start` is the repair path. Doctor stays read-only.
- **`--verbose` flag** — every check is already shown.
- **`--category` filter** — not enough checks to warrant it.
- **Concurrent checks** — not worth the complexity for <20 fast checks.

# Port Management Feature — Design Spec

## Summary

Add port inspection and process management to Outport via a new `outport ports` command. Outport already allocates deterministic ports and checks liveness — this feature adds the ability to see what's actually running on those ports, detect orphaned processes, and kill them. Inspired by [port-whisperer](https://github.com/LarsenCundric/port-whisperer), scoped to work as a natural extension of Outport's existing port knowledge.

## Motivation

Outport manages port allocation but currently can't tell you what process is on a port, whether it's orphaned, or help you kill it. When a crashed dev server holds a port, the user has to manually run `lsof`, find the PID, and kill it. Since Outport is already the port tool, it should handle this end-to-end.

## Command Structure

```
outport ports                  # Project ports (in project dir) or all Outport ports (outside)
outport ports --all            # Full machine scan — every listening TCP port
outport ports --json           # Machine-readable output (standard envelope)
outport ports kill <target>    # Kill by service name or port number
outport ports kill --orphans   # Kill all orphaned dev processes
outport ports kill --force     # Skip confirmation prompt
```

`outport ports` is a top-level command (GroupID: "project") with `kill` as a subcommand.

### Default Behavior by Context

| Context              | Default                                      | `--all`            |
|----------------------|----------------------------------------------|--------------------|
| Inside project dir   | This project's ports with live process info   | Full machine scan  |
| Outside project dir  | All Outport-managed ports with live process info | Full machine scan  |

`kill` works from anywhere — by service name (if in a project), by port number (always), or `--orphans` (always).

## Architecture

### New Package: `internal/portinfo`

Handles all system-level port scanning and process inspection. Keeps lsof/ps details out of the CLI layer.

**Exported types:**

```go
type ProcessInfo struct {
    PID         int
    PPID        int
    Name        string       // process name (e.g., "node")
    Command     string       // full command string
    Port        int
    RSS         int64        // resident memory in bytes
    StartTime   time.Time
    State       string       // process state from ps (e.g., "S", "Z")
    CWD         string       // working directory
    Project     string       // detected project name (from CWD markers)
    Framework   string       // detected framework (e.g., "Next.js", "Rails")
    IsOrphan    bool         // ppid=1 + dev process allowlist
    IsZombie    bool         // state contains "Z"
}
```

**Exported functions:**

```go
// Scan discovers all listening TCP ports and returns process info for each.
// The scanner parameter allows tests to inject canned output.
func Scan(scanner Scanner) ([]ProcessInfo, error)

// ScanPorts scans only the specified ports. Used for project-scoped views.
func ScanPorts(ports []int, scanner Scanner) ([]ProcessInfo, error)

// Kill sends SIGTERM to the given PID. Returns an error if the process
// cannot be killed (permission denied, PID 1, etc.).
func Kill(pid int) error
```

**Scanner interface (for testability):**

```go
type Scanner interface {
    // ListeningPorts returns raw lsof output for all listening TCP ports.
    ListeningPorts() (string, error)
    // ProcessInfo returns raw ps output for the given PIDs.
    ProcessInfo(pids []int) (string, error)
    // WorkingDirs returns raw lsof CWD output for the given PIDs.
    WorkingDirs(pids []int) (string, error)
}
```

A `SystemScanner` struct implements the real system calls. Tests inject a `FakeScanner` with canned output.

### System Commands Used

**Port discovery:**
```
lsof -iTCP -sTCP:LISTEN -P -n
```
Returns all listening TCP ports with PID and process name. Works on both macOS and Linux.

**Process details (batched):**
```
ps -p <pid1>,<pid2>,... -o pid=,ppid=,stat=,rss=,lstart=,command=
```
Single call for all PIDs. Returns parent PID, state, RSS memory, start time, full command.

**Working directories (batched):**
```
lsof -a -d cwd -p <pid1>,<pid2>,...
```
Resolves each process's CWD for framework/project detection.

### Framework Detection

From the process's CWD, walk up the directory tree (max 15 levels) looking for project root markers:

| File | Framework/Language |
|---|---|
| `package.json` | Parse deps for Next.js, Nuxt, Vite, Angular, Express, etc. |
| `go.mod` | Go |
| `Gemfile` | Ruby (check for Rails, Sinatra) |
| `Cargo.toml` | Rust |
| `pyproject.toml` / `requirements.txt` | Python (check for Django, Flask, FastAPI) |
| `pom.xml` / `build.gradle` | Java |

Fallback: map process name to language (node → Node.js, ruby → Ruby, python → Python, etc.).

### Orphan Detection

A process is flagged as orphaned when BOTH conditions are met:
1. `ppid == 1` (adopted by init/launchd — parent died)
2. Process name matches a dev-process allowlist: node, ruby, python, python3, go, cargo, deno, bun, java, php, elixir, beam.smp, dotnet

This avoids false positives on system daemons (which also have ppid=1 but aren't dev processes).

Zombie detection: process state contains "Z".

## Output

### Default View (project-scoped)

```
myapp [main]

    web               PORT                 → 13542  ✓ up    https://myapp.test
                      PID 48291 · node (next dev) · 142 MB · 2h 14m

    api               API_PORT             → 28901  ✗ down
```

Same layout as `outport up` / `outport status`, with a process detail line beneath each service that has a listening process. No process line when the port is down.

### All Outport Ports (outside project dir)

Same as `outport system status --check` layout but with process detail lines.

### Full Machine Scan (`--all`)

```
Outport managed:
    myapp/web         → 13542  ✓ up    https://myapp.test
                        PID 48291 · node (next dev) · 142 MB · 2h 14m

    myapp/api         → 28901  ✗ down

Other:
    3000    node (next dev)         PID 51002 · my-side-project (Next.js) · 98 MB · 45m
    5432    postgres                PID 412   · PostgreSQL
    6379    redis-server            PID 538   · Redis
```

Outport-managed ports get enriched project/service labels. Non-Outport ports show port, process name, PID, detected project/framework, memory, uptime. Orphaned processes get a warning marker.

### Kill Output

```
$ outport ports kill web
Kill process on port 13542?
  PID 48291 · node (next dev) · myapp · 142 MB · 2h 14m
  Confirm [y/N]: y
✓ Killed PID 48291 (SIGTERM)
```

### JSON Output

Standard envelope: `{"ok": true, "data": [...], "summary": "..."}`.

**Project-scoped (default in project dir):**

```json
{
  "ok": true,
  "data": {
    "project": "myapp",
    "instance": "main",
    "ports": [
      {
        "port": 13542,
        "service": "web",
        "hostname": "myapp.test",
        "url": "https://myapp.test",
        "up": true,
        "process": {
          "pid": 48291,
          "ppid": 1042,
          "name": "node",
          "command": "node .next/standalone/server.js",
          "rss_bytes": 148897792,
          "uptime_seconds": 8040,
          "cwd": "/Users/steve/src/myapp",
          "project": "myapp",
          "framework": "Next.js",
          "is_orphan": false,
          "is_zombie": false
        }
      }
    ]
  },
  "summary": "2 ports, 1 up"
}
```

**Full scan (`--all`) and all-Outport (outside project dir):**

Every port entry uses the same structure. Outport-managed ports include `service`, `hostname`, `url`, and `registry_key` fields. Non-Outport ports omit those fields.

```json
{
  "ok": true,
  "data": {
    "managed": [
      {
        "registry_key": "myapp/main",
        "port": 13542,
        "service": "web",
        "hostname": "myapp.test",
        "url": "https://myapp.test",
        "up": true,
        "process": { "..." : "same as above" }
      }
    ],
    "other": [
      {
        "port": 3000,
        "up": true,
        "process": {
          "pid": 51002,
          "ppid": 1,
          "name": "node",
          "command": "node server.js",
          "rss_bytes": 102760448,
          "uptime_seconds": 2700,
          "cwd": "/Users/steve/src/side-project",
          "project": "side-project",
          "framework": "Next.js",
          "is_orphan": true,
          "is_zombie": false
        }
      }
    ]
  },
  "summary": "3 ports (2 managed, 1 other), 2 up"
}
```

## Kill Behavior

### Target Resolution

1. **Service name** → look up port from current project's registry (e.g., `outport ports kill web`). Requires project context.
2. **Port number** → use directly (e.g., `outport ports kill 3000`). Works anywhere.
3. **`--orphans`** → scan and kill all orphaned dev processes. Works anywhere.

### Signal Handling

SIGTERM only. If the process survives (checked after 3 seconds), print a suggestion: `Process still running. Try: sudo kill -9 <pid>`. No auto-escalation to SIGKILL.

### Confirmation

y/N prompt by default showing full process details. `--force` skips the prompt.

### Multiple PIDs on One Port

If multiple processes listen on the same port (parent + forked children), show all and kill the parent PID (lowest PID in the group).

### Safety Checks

Refuse to kill and show an error for:
- PID 1 (init/launchd)
- PID 0 (kernel)
- The Outport daemon process itself
- System processes (ppid=0)

## Doctor Integration

Add a new check to `doctor.ProjectChecks()`:

**"Orphaned processes"** (category: "Ports") — Scans all of the current project's allocated ports for orphaned or zombie processes. Status: Warn if any found, Pass otherwise. Fix suggestion: `Run: outport ports kill --orphans`.

## Testing

### `internal/portinfo`

- `Scanner` interface allows injecting canned lsof/ps output — no real process spawning in tests.
- Table-driven tests for: lsof output parsing (various formats, edge cases), ps output parsing, orphan detection (ppid=1 + allowlist combinations), zombie detection, framework detection from project markers, framework detection from package.json deps, empty/malformed output, permission errors.

### `cmd/ports.go` and `cmd/ports_kill.go`

- JSON envelope output validation.
- Flag validation (service name vs port number vs --orphans).
- Kill safety checks (refuse PID 1, refuse daemon PID).
- `--force` skips prompt.
- Context handling (inside vs outside project directory).

### E2E (BATS)

Smoke test: `outport ports --json` returns valid JSON with the envelope structure.

## Phases

**Phase 1 — `internal/portinfo` + `outport ports`**: Core scanning, parsing, process info, framework detection. The command with default/`--all` views and `--json`.

**Phase 2 — `outport ports kill`**: Kill subcommand with service name/port number/`--orphans` targeting. Confirmation, `--force`, safety checks.

**Phase 3 — Doctor integration**: Orphan detection check in `outport doctor`.

Phases 1–3 ship together on one feature branch. Phase 4 (dashboard enrichment) is a separate future spec.

## Files to Create/Modify

### New files
- `internal/portinfo/portinfo.go` — types, Scanner interface, Scan/ScanPorts/Kill functions
- `internal/portinfo/parse.go` — lsof/ps output parsing
- `internal/portinfo/detect.go` — framework detection, orphan detection
- `internal/portinfo/portinfo_test.go` — unit tests
- `cmd/ports.go` — `outport ports` command
- `cmd/ports_kill.go` — `outport ports kill` subcommand

### Modified files
- `cmd/root.go` — register `portsCmd`
- `internal/doctor/checks.go` — add orphan check
- `cmd/doctor.go` — wire orphan check into project checks

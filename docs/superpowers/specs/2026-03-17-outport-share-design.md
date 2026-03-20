# outport share — Tunneling Dev Services to Public URLs

**Date:** 2026-03-17
**Issue:** #16
**Status:** Design approved, pending implementation

## Problem

Sometimes you need to access your dev app from outside your local network — showing a client, testing webhook callbacks, or testing on a phone. Local `.test` domains and the reverse proxy only work on the dev machine. There's no way to get a public URL for a locally running service.

## Solution

`outport share` tunnels HTTP services to public URLs via Cloudflare quick tunnels. It shells out to `cloudflared`, captures the generated URLs, and displays them. The command blocks until Ctrl+C.

## Usage

```bash
outport share              # Tunnel all HTTP services
outport share web          # Tunnel a specific service
outport share web vite     # Tunnel specific services
outport share --json       # Machine-readable output (for agents/scripts)
```

## Architecture

### Package Structure

```
internal/tunnel/
  provider.go      — Provider interface + Tunnel type
  tunnel.go        — Manager: coordinates multiple tunnels (start all, stop all, collect URLs)

internal/tunnel/cloudflare/
  cloudflare.go    — Cloudflare quick tunnel provider (shells out to cloudflared)

cmd/
  share.go         — The outport share command
```

### Provider Interface

A lightweight abstraction to insulate Outport from any single tunnel provider. Motivated by the Valet/ngrok cautionary tale — ngrok changed their terms to require signup after Valet had hardcoded it as the only option.

```go
// internal/tunnel/provider.go

type Provider interface {
    Name() string
    CheckAvailable() error
    Start(ctx context.Context, port int) (*Tunnel, error)
}

type Tunnel struct {
    Service  string
    URL      string
    Port     int
    Stop     func() error
}
```

The interface is intentionally minimal. A provider needs to:
1. Report its name (for error messages)
2. Check if its binary/dependency is available
3. Start a tunnel for a given port, returning a URL and a stop function

### Tunnel Manager

`tunnel.Manager` orchestrates multiple tunnels:
- Takes a `Provider` and a map of service names to ports
- Starts all tunnels concurrently (one goroutine per service)
- Waits for all URLs with a 15-second timeout
- All-or-nothing: if any tunnel fails, stops the ones that succeeded and returns error
- Returns `[]Tunnel` on success

The manager owns the concurrency. The provider's `Start()` blocks until it has a URL or the context is cancelled. The manager creates per-tunnel contexts derived from a parent, so cancelling the parent tears everything down.

### Cloudflare Provider

Shells out to `cloudflared tunnel --url http://localhost:{port}`.

**CheckAvailable():** Calls `exec.LookPath("cloudflared")`. Returns error with install instructions if not found.

**Start():**
1. Build and start the subprocess
2. Pipe stderr into a scanner goroutine
3. Scan for URL matching `https://[-\w]+\.trycloudflare\.com`
4. Return `&Tunnel{URL: url, Stop: ...}` when URL captured
5. On context cancellation or timeout: kill process, return error
6. On process crash: return error with last few stderr lines for debugging

**Stop function:** Sends SIGTERM, waits up to 3 seconds for graceful exit, then SIGKILL.

**Verified against cloudflared source (2026.3.0):**
- URL printed to stderr via zerolog Info level, inside an ASCII box
- All log output goes to stderr; nothing goes to stdout
- Signal handling: SIGTERM/SIGINT trigger graceful shutdown with 30-second grace period
- Quick tunnel URL obtained by POST to `https://api.trycloudflare.com/tunnel`
- URL format: `https://{four-hyphenated-words}.trycloudflare.com`
- URL appears ~7 seconds after process start (API call latency), hence 15-second timeout

### Command Flow

1. `loadProjectContext()` — load config, registry, instance
2. Look up allocation from registry — error if not found
3. Filter services: if args given, validate they exist and have `protocol: http/https`. If no args, collect all HTTP services. Error if none found.
4. Check `cloudflared` availability via provider
5. Create tunnel manager, start all tunnels
6. Print results (styled or JSON), flush immediately
7. Block on `signal.NotifyContext(SIGTERM, SIGINT)`
8. Manager stops all tunnels, command exits

## Output

### Styled (default)

```
Sharing 2 services:

  web     https://soft-property-mas-trees.trycloudflare.com → localhost:3000
  vite    https://seasonal-deck-organisms-sf.trycloudflare.com → localhost:5173

Press Ctrl+C to stop sharing.
```

### JSON (`--json`)

```json
{
  "tunnels": [
    {"service": "web", "url": "https://soft-property-mas-trees.trycloudflare.com", "port": 3000},
    {"service": "vite", "url": "https://seasonal-deck-organisms-sf.trycloudflare.com", "port": 5173}
  ]
}
```

JSON is flushed to stdout immediately after tunnel URLs are captured, so a consuming process (agent, script) can read it while tunnels are still running.

## Error Handling

### Precondition Errors (before starting tunnels)

| Condition | Error |
|-----------|-------|
| No `outport.yml` found | `no outport configuration found. Run 'outport init' first` |
| No registry entry | `no port allocations found. Run 'outport apply' first` |
| Named service doesn't exist | `unknown service "foo"` (FlagError, shows usage) |
| Named service has no protocol | `service "redis" has no protocol and cannot be shared` |
| No HTTP services in project | `no shareable services found. Add 'protocol: http' to a service in outport.yml` |
| `cloudflared` not installed | `cloudflared not found. Install with: brew install cloudflared` |

### Runtime Errors (during tunnel startup)

| Condition | Behavior |
|-----------|----------|
| Tunnel times out (15s) | Stop all successful tunnels, exit with: `timed out waiting for tunnel URL for service "web"` |
| `cloudflared` crashes | All-or-nothing: stop others, exit with error + last stderr lines |
| All tunnels fail | Exit with error + stderr hint |

### Shutdown

Ctrl+C (SIGINT) and SIGTERM both trigger graceful shutdown. Stop all tunnels, wait briefly, exit 0.

## Testing Strategy

### Mock Provider (for manager tests)

A mock provider that returns preset URLs, errors, or simulates timeouts. Tests:
- All tunnels succeed
- One tunnel fails (verify all-or-nothing teardown)
- Timeout handling
- Context cancellation

### Cloudflare Provider Unit Tests

- `CheckAvailable()` with fake PATH
- URL parsing extracted into a testable function, tested against real captured cloudflared output format

### Command-Level Tests

Following existing `cmd/cmd_test.go` patterns:
- No registry entry → correct error
- No HTTP services → correct error
- Named service doesn't exist → correct error (FlagError)
- `cloudflared` not in PATH → correct error

### No Integration Tests

Quick tunnels require internet access and take 7+ seconds. Not suitable for `just test`. We test the seams: URL parsing against real output, manager orchestration against mock provider, command validation against real config/registry.

## Future Considerations (not in scope)

- **#17 Multi-service env rewriting** — writing tunnel URLs back into `.env` so services discover each other. The provider interface and manager architecture support this naturally.
- **#15 QR codes** — `outport share --qr` to display QR codes for tunnel URLs.
- **Provider selection** — `--provider` flag or config field if a second provider is ever needed.
- **Named tunnels** — Cloudflare account-based tunnels with stable URLs for more persistent sharing.

## Research Sources

- Cloudflared source code (v2026.3.0): `github.com/cloudflare/cloudflared`
- Live test of `cloudflared tunnel --url http://localhost:8080` on macOS arm64
- Laravel Valet `share` command implementation (ngrok → multi-provider evolution)
- DDEV `share` command implementation (bash script provider system)
- Alternatives evaluated and rejected: ngrok, bore, localhost.run, Tailscale Funnel, localtunnel, Pinggy, zrok, frp

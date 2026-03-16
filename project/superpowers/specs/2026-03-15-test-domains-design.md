# Local DNS + Reverse Proxy for .test Domains

**Issue:** #13
**Date:** 2026-03-15
**Status:** Design approved, pending implementation plan

## Problem

After `outport apply`, services are still accessed via `localhost:24920` — hash-based port numbers that are intentionally non-memorable (to support agentic workflows where multiple instances run simultaneously). This creates two problems:

1. **Cookie collision across instances.** Two instances of the same app on different ports share `localhost` cookies. Browsers can't distinguish `localhost:24920` from `localhost:31847` for cookie isolation. The workaround (incognito windows, separate browsers) breaks down when 5 agents are running parallel instances.

2. **Poor developer experience.** Remembering hash-based port numbers defeats the purpose of port orchestration — ports should disappear from the developer's mental model entirely.

## Solution

Give every HTTP service a `.test` hostname via local DNS + reverse proxy. Main instances get clean names (`unio.test`), additional instances get auto-generated codes (`unio-bkrm.test`). Each instance gets automatic cookie isolation because each has a unique hostname.

## Design

### 1. Instance Model (replaces worktree detection)

The registry becomes the single source of truth for instance identity. The `internal/worktree` package is removed entirely — no backward compatibility shims.

**Instance resolution in `outport apply`:**

1. Load `.outport.yml` → get project name. Resolve the config directory (the directory containing `.outport.yml`, found via `config.FindDir()`). This resolved config directory — not the raw cwd — is what gets matched against `project_dir` in the registry.
2. Search registry for any entry where `project_dir` matches the resolved config directory.
   - **Found** → use that instance name. Done.
3. Not found → search registry for any other instances of this project (matching project name).
   - **None exist** → register as `"main"` automatically.
   - **Others exist** → auto-generate a 4-char consonant code (e.g., `bkrm`), register it, print: *"Registered as unio-bkrm. Use `outport rename bkrm <name>` to rename."*

**Instance code generation:**

- Alphabet: `bcdfghjkmnpqrstvwxz` (19 consonants — no vowels, no ambiguous `l`)
- 4 characters → ~130,000 combinations
- No vowels makes offensive words virtually impossible
- Generated randomly, collision-checked against existing instances in the registry
- Once assigned, the code is stable — it persists in the registry and does not change unless explicitly renamed

**New commands for instance management:**

- `outport rename <old> <new>` — rename an instance. Must be run from a directory belonging to the target project (so the project name can be resolved from `.outport.yml`). Updates registry key, recomputes hostnames for the renamed instance, and re-merges all `.env` files for that instance (so derived values referencing `${service.url}` or `${service.hostname}` get updated). The new name must not collide with an existing instance of the same project. Ports are unchanged.
- `outport promote` — run from a non-main instance directory. Swaps it to `"main"`, demotes current main to an auto-generated code. If no current main exists (deleted/gc'd), the instance simply becomes main with no swap. Both instances' `.env` files are re-merged to reflect the new hostnames. Ports stay the same, only hostnames and registry keys change.

### 2. Registry Extension

The registry expands to store hostnames and protocols alongside port allocations. This makes it the single source of truth for the daemon's routing table — the daemon never reads `.outport.yml` files.

**Current format:**

```json
{
  "unio/main": {
    "project_dir": "/src/unio",
    "ports": { "rails": 24920, "portal": 21133 }
  }
}
```

**New format:**

```json
{
  "unio/main": {
    "project_dir": "/src/unio",
    "ports": { "rails": 24920, "portal": 21133 },
    "hostnames": { "rails": "unio.test", "portal": "portal.unio.test" },
    "protocols": { "rails": "http", "portal": "http" }
  }
}
```

**Hostname computation (performed by `outport apply`):**

- Each HTTP/HTTPS service declares its `hostname` in `.outport.yml` (e.g., `hostname: unio` or `hostname: portal.unio`). The `hostname` field is now the hostname stem — it does not include the `.test` suffix. This is a breaking change from the previous convention where `hostname` could be set to values like `myapp.localhost`.
- For the `"main"` instance, the hostname is the stem with `.test` appended: `unio.test`, `portal.unio.test`.
- For other instances, the project name in the hostname stem is replaced with `{project}-{instance}`, then `.test` is appended. Replacement targets the rightmost occurrence of the project name in the stem to handle subdomain patterns correctly (e.g., `portal.unio` → replace `unio` → `portal.unio-bkrm` → `portal.unio-bkrm.test`).
- Non-HTTP services (postgres, redis) get no hostname entry.

**Hostname validation:**

- The `hostname` field must contain only characters valid in DNS hostnames: lowercase alphanumeric, hyphens, and dots (for subdomains). Underscores and other special characters are rejected at config load time.
- Hostnames must contain the project name (from `name:` field) as a segment, since instance suffixing depends on finding and replacing it.
- `outport apply` checks for hostname uniqueness across all registry entries. If a computed hostname collides with one from a different project, apply fails with a clear error: *"Hostname 'foo.test' conflicts with project 'bar' (instance 'main')."*

**Backward compatibility:** Old registry entries without `hostnames` or `protocols` fields are handled gracefully — Go's `json.Unmarshal` leaves them as nil maps. The daemon skips entries with nil hostname maps. Running `outport apply` in each project backfills the new fields. No explicit migration step is needed.

**Validation:** A service that declares `hostname` without `protocol: http` or `protocol: https` is a config error, caught at config load time.

### 3. Daemon Architecture

A single long-running process containing two servers:

**DNS server** (embedded, `miekg/dns` library):
- Listens on UDP 127.0.0.1:15353
- Answers all `*.test` A-record queries with `127.0.0.1` (TTL 60s — kept short for future flexibility, e.g., if non-loopback addresses are ever supported)
- Returns NXDOMAIN for non-`.test` queries
- Port 15353 falls within the allocator's range (10000–39999) and must be added to the reserved ports set during allocation so it is never assigned to a service

**HTTP reverse proxy** (`net/http/httputil.ReverseProxy`):
- Receives traffic on port 80 via launchd socket activation (process never runs as root)
- Maintains a `map[string]int` routing table: hostname → port
- On request: extracts `Host` header, looks up port, proxies to `127.0.0.1:{port}`
- Fully transparent — passes all headers, cookies, auth tokens, request bodies untouched
- WebSocket support: detects `Connection: Upgrade` + `Upgrade: websocket`, hijacks connection, relays bidirectionally with `io.Copy`
- **Known hostname, backend down** → returns helpful error page: *"unio.test is not running. Start your app and try again."*
- **Unknown hostname** (not in routing table) → returns error page: *"No project is configured for random.test. Run `outport apply` with a matching hostname."*

**Route table management:**
- On startup, reads `registry.json` and builds routing table from all entries with HTTP/HTTPS protocols
- Watches the registry directory (not the file itself) via fsnotify, filtering for `registry.json` changes — this avoids the known issue where atomic writes (temp file + rename) produce inconsistent events when watching a single file on macOS with kqueue
- Route table rebuilds use a full swap: build a new map, then swap the pointer under a write lock. The write lock is never held during the registry read/parse operation.
- If `registry.json` is deleted while the daemon is running, the daemon retains the last-known routing table and logs a warning. Routes remain active until the file reappears and is read.
- Thread-safe via `sync.RWMutex`

**New package:** `internal/daemon` — contains DNS server, HTTP proxy, route table builder, and registry watcher.

### 4. Daemon Lifecycle (macOS)

Uses the puma-dev pattern: launchd socket activation. launchd (root) binds port 80 and passes the file descriptor to the daemon process running as the current user. The daemon itself never runs as root.

**`launch_activate_socket()` implementation:** This macOS API requires cgo (pure Go cannot call it). This impacts the build pipeline — GoReleaser must build with `CGO_ENABLED=1` for macOS targets. Cross-compilation for macOS from Linux CI is not possible with cgo; builds must run on macOS (GitHub Actions macOS runners, or local builds). The GoReleaser config needs updating to account for this. Linux builds remain pure Go and are unaffected.

**One-time setup (`outport setup`):**

The command uses `sudo` only for the step that requires it:
1. Installs LaunchAgent plist to `~/Library/LaunchAgents/dev.outport.daemon.plist` with socket activation for port 80 (no sudo — user-owned file)
2. Runs `sudo` to create `/etc/resolver/test` — the command prompts for the password via the system sudo mechanism. Only this specific write operation runs elevated.
3. Checks if port 80 is already in use before loading the agent. If occupied, reports the conflicting process and exits with an error: *"Port 80 is in use by {process}. Stop it and re-run `outport setup`."*
4. Loads the agent (daemon starts immediately)

**Teardown (`outport teardown`):**
1. Unloads LaunchAgent
2. Removes plist
3. Runs `sudo` to remove `/etc/resolver/test`

**Manual control:**
- `outport up` / `outport down` — start/stop daemon without removing the LaunchAgent. Thin wrappers around `launchctl load/unload`.

**Auto-start:** After setup, the daemon starts automatically at user login. No manual intervention needed day-to-day.

**Platform boundary:** All macOS-specific code (launchd, resolver) lives behind a `platform.Setup()` / `platform.Install()` interface. Linux support is a separate future spec — the core daemon logic (DNS server, proxy, route table) is platform-agnostic.

### 5. CLI Changes

**New commands:**

| Command | Purpose | Sudo |
|---------|---------|------|
| `outport setup` | One-time: install LaunchAgent, create resolver file, start daemon | Partial (sudo for `/etc/resolver/` only) |
| `outport teardown` | Reverse of setup: unload LaunchAgent, remove resolver | Partial (sudo for `/etc/resolver/` only) |
| `outport up` | Start daemon manually | No |
| `outport down` | Stop daemon manually | No |
| `outport rename <old> <new>` | Rename an instance | No |
| `outport promote` | Swap current instance to "main" | No |

**Modified commands:**

| Command | Change |
|---------|--------|
| `outport apply` | Registry-based instance resolution; computes/stores hostnames + protocols; shows hostname info; prints setup hint if not configured |
| `outport ports` | Shows hostnames alongside ports when setup is active |
| `outport status` | Shows hostnames in global project list |
| `outport open` | Uses `.test` URLs when setup is active, falls back to `localhost:PORT` |
| `outport unapply` | No functional change needed beyond handling new registry fields |
| `outport gc` | Unchanged — already removes entries where `project_dir` doesn't exist. New fields are cleaned up with the entry. |

All new and modified commands support `--json` output. JSON schemas for new commands will be defined during implementation planning (following the existing pattern of paired `print*Styled()` / `print*JSON()` functions).

**Removed:**
- `internal/worktree` package — replaced entirely by registry-based instance resolution.

**Fix required:** The existing error message in `open.go` that says *"Run 'outport up' first"* must be corrected — it should say *"Run 'outport apply' first."* This pre-existing bug becomes actively misleading now that `outport up` means "start the daemon."

### 6. Config Changes

**`.outport.yml` — no new fields.** The existing `hostname` and `protocol` fields express everything needed. Two changes:

1. **Tightened validation:** `hostname` without `protocol: http/https` is now a config error.
2. **Breaking change to `hostname` semantics:** The `hostname` field is now a stem (e.g., `unio`, `portal.unio`) rather than a full hostname (e.g., `unio.localhost`). The `.test` suffix and instance suffixing are computed by Outport. Existing configs that used `hostname: myapp.localhost` must be updated to `hostname: myapp`.

**Example config (Unio monorepo):**

```yaml
name: unio
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    hostname: unio
    env_file:
      - backend/.env
      - frontend/apps/main/.env
      - frontend/apps/portal/.env
  frontend_main:
    env_var: MAIN_PORT
    protocol: http
    hostname: main.unio
    env_file:
      - frontend/apps/main/.env
      - backend/.env
  frontend_portal:
    env_var: PORTAL_PORT
    protocol: http
    hostname: portal.unio
    env_file:
      - frontend/apps/portal/.env
      - backend/.env
  postgres:
    env_var: DB_PORT
    env_file: backend/.env
  redis:
    env_var: REDIS_PORT
    env_file: backend/.env
```

### 7. Template Modifier System

Extends the existing `${service.field}` template syntax with `${service.field:modifier}` for context-dependent URL generation.

**Implementation note:** The current template regex (`\$\{(\w+)\.(\w+)\}` in `config.go`) must be extended to capture an optional modifier group: `\$\{(\w+)\.(\w+)(?::(\w+))?\}`. The `validFields` map must be extended to include `url` alongside the existing `port` and `hostname`.

**New and changed template fields:**

| Template | Resolves to | Notes |
|----------|------------|-------|
| `${rails.port}` | `24920` | Unchanged |
| `${rails.hostname}` | `unio.test` | **Changed** — now resolves to instance-aware `.test` hostname instead of the raw config value. For services without a `hostname` declaration, this field is unavailable (config validation will catch references to it in derived values). |
| `${rails.url}` | `http://unio.test` | **New** — full proxied URL, no port (port 80 via proxy) |
| `${rails.url:direct}` | `http://localhost:24920` | **New** — direct localhost URL with port, for server-to-server communication |

**The `:direct` modifier** resolves to a `localhost:{port}` URL, bypassing the proxy. This is essential for server-to-server communication (e.g., Nuxt SSR calling the Rails API) where requests originate from the same machine, not from a browser.

**Modifier parsing:** The template resolver splits on `:` after the field name. Only one modifier is supported initially (`:direct`), but the parser handles the pattern generically for future extensibility. An unrecognized modifier is a config validation error.

**Example — Unio derived values with modifiers:**

```yaml
derived:
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/api/v1"
      - file: frontend/apps/portal/.env
        value: "${rails.url:direct}/portal/api/v1"

  CORE_CORS_ORIGINS:
    value: "${frontend_main.url},${frontend_portal.url}"
    env_file: backend/.env

  CORE_FRONTEND_URL:
    value: "${frontend_main.url}"
    env_file: backend/.env

  SHRINE_ASSET_HOST:
    value: "${rails.url}"
    env_file: backend/.env
```

### 8. Graceful Degradation

The DNS + proxy layer is opt-in. All port orchestration functionality works without `outport setup`.

**Behavior without setup:**

- `outport apply` allocates ports, writes `.env`, computes and stores hostnames in registry. Shows a one-time hint: *"Hostnames computed but not routable. Run `sudo outport setup` to enable .test domains."*
- `outport ports` shows ports only (no hostnames).
- `outport open` opens `http://localhost:PORT`.
- `${service.url}` and `${service.hostname}` resolve to `.test` hostnames regardless of setup state. This means `.env` files contain `.test` URLs even before setup. This is intentional — the user is expected to run `outport setup` before using hostname-dependent features. If a project needs to work without setup, it should use `${service.url:direct}` for all URL templates.

**Detection:** Commands check for the existence of `/etc/resolver/test` and `~/Library/LaunchAgents/dev.outport.daemon.plist` to determine whether setup has been completed.

**After running setup:** Every project that has already been `apply`'d immediately gets working hostnames — no need to re-run `apply` across projects. The daemon reads the registry and builds routes from what's already there.

### 9. Hostname Conventions

**Subdomain model for multi-service projects:**

| Service | Main instance | Instance `bkrm` |
|---------|--------------|------------------|
| rails (`hostname: unio`) | `unio.test` | `unio-bkrm.test` |
| portal (`hostname: portal.unio`) | `portal.unio.test` | `portal.unio-bkrm.test` |
| postgres (no hostname) | — | — |

**Instance suffixing rule:** For non-main instances, find the rightmost occurrence of the project name in the hostname stem and replace it with `{project}-{instance}`, then append `.test`. This targets the rightmost occurrence because the project name appears at the root level of the hostname hierarchy, while subdomains (service names) precede it.

Examples with project name `unio`:
- `unio` → `unio-bkrm` → `unio-bkrm.test`
- `portal.unio` → `portal.unio-bkrm` → `portal.unio-bkrm.test`
- `api.portal.unio` → `api.portal.unio-bkrm` → `api.portal.unio-bkrm.test`

**Wildcard DNS:** The embedded DNS server answers all `*.test` queries with `127.0.0.1`, so any depth of subdomains works without additional configuration.

## Scope

**In scope:**
- Instance model replacing worktree detection
- Registry extension with hostnames and protocols
- Embedded DNS server (`miekg/dns`)
- HTTP reverse proxy with WebSocket support
- launchd socket activation (macOS)
- CLI commands: `setup`, `teardown`, `up`, `down`, `rename`, `promote`
- Template modifier system (`:direct`)
- Graceful degradation without setup

**Out of scope:**
- Linux support (separate spec — platform interface designed for it)
- SSL/HTTPS certificates (v3 milestone)
- Automatic app startup (Outport routes to ports, doesn't manage processes)
- Configurable TLD (locked to `.test`)

## Dependencies

- `miekg/dns` — embedded DNS server
- `fsnotify/fsnotify` — registry file watching
- macOS `launch_activate_socket()` — via cgo for launchd socket activation (impacts GoReleaser build config for macOS targets)

## Related Issues

- #26 — Investigate checking if ports are already in use before allocating (discovered during this design)

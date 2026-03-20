# Research Notes

## Existing Tools Reviewed

### dot-test (Obi Fernandez)
- **Source:** ~/src/vendor/dot-test (Go, zero dependencies)
- **What it does:** Scans for Rails projects, assigns sequential ports (from 3000), writes `PORT=` to `.env`, runs DNS server (UDP 15353) + HTTP reverse proxy (port 80) for `*.test` domains
- **Detection:** Looks for `config/application.rb` (Rails only)
- **Port detection:** Checks `bin/dev`, `Procfile.dev`, `docker-compose.yml` for hardcoded ports
- **DNS:** Creates `/etc/resolver/test` on macOS pointing to local DNS server
- **Proxy:** Go's `net/http/httputil.ReverseProxy`, maps hostname → port
- **Limitations:** Rails-only, HTTP-only (no SSL), no backing service ports, no worktree awareness, sequential assignment (fragile), no conflict detection

### Kiso bin/worktree (Steve Clarke)
- **Source:** ~/src/kiso/bin/worktree (Bash)
- **What it does:** MD5 hash of worktree name → deterministic port offset within range
- **Formula:** `md5(worktree_name) % 500` + base offset
- **Ranges:** Lookbook 4101-4600, Docs 4601-5100
- **Integration:** Writes env vars, Procfile.dev uses `${PORT:-default}`
- **Limitations:** Project-specific (baked into Kiso), bash, two services only

### Portree
- FNV32 hash-based port assignment with per-service ranges
- TUI dashboard for managing services
- Automatic subdomain routing via reverse proxy
- HTTPS proxy with auto-generated certs
- Linear probing on hash collisions
- **No Docker/backing service awareness**

### Portless (Vercel Labs)
- Random ports (4000-4999) + reverse proxy
- `myapp.localhost` naming, worktree-aware with branch subdomains
- Has a Rust rewrite (portless-rs)
- **No backing service awareness**

### worktree-compose
- Auto-isolates Docker stacks per worktree
- Port formula: `20000 + default_port + index`
- Syncs compose files, injects port overrides into `.env`
- Has MCP server for AI agents
- **Best for Docker-heavy setups, single repo only**

### devports (Ben Dechrai)
- Template-based config rendering with managed port registry
- Define services and templates, create allocations per worktree
- Explicitly mentions LLM agent use cases
- Language-agnostic

### Building Block Libraries
- **acquire-port** (npm) — deterministic port from project name, falls to next available if occupied
- **hash-to-port** (npm) — pure hash function, string → port
- **hashport** (Python) — text-to-port hashing, dynamic range (49152-65535)

## Port Allocation Strategies

| Strategy | Pros | Cons |
|----------|------|------|
| **Sequential** (dot-test) | Simple, predictable | Fragile with additions/removals, no worktree support |
| **Hash-based** (Kiso, Portree) | Deterministic, stable across restarts | Possible collisions, needs range management |
| **Random + persist** | No collisions if checked | Not deterministic, needs registry |
| **Formula** (worktree-compose) | Predictable, spread out | Large port numbers (20000+), rigid |
| **Find free + persist** | Always works | Needs cleanup, not deterministic |

**Recommended approach:** Hash-based with collision detection and fallback. Deterministic means same worktree always gets same ports (nice for bookmarks, muscle memory), collision detection handles the rare overlap.

## Integration Patterns

### .env is the universal contract
- Docker Compose: reads `.env` automatically
- Foreman/Overmind: reads `.env` (or `.env.local`)
- Rails (dotenv-rails): reads `.env`
- Nuxt: reads `.env`
- Most frameworks have dotenv support

### What needs to read the ports
1. **Process manager** (Foreman/Overmind) — `PORT` env var
2. **Docker Compose** — `${DATABASE_PORT:-5432}` in compose.yml
3. **Rails config** — `database.yml` reads env vars
4. **Framework config** — nuxt.config.ts, etc.

### macOS DNS setup
- `/etc/resolver/test` file with `nameserver 127.0.0.1` + `port 15353`
- One-time setup, persists across reboots
- Tells macOS to route all `*.test` queries to local DNS server

## SSL — Local CA, not Let's Encrypt (2026-03-15)

### DNS-PERSIST-01 is a dead end for `.test`

Let's Encrypt won't issue certificates for IANA-reserved TLDs like `.test`. DNS-PERSIST-01 is only useful for real domains (potentially `myapp.outport.app` for tunneling later). Not viable for local dev.

### Local CA is the right approach

Validated by Portless's working implementation (see below). The path:

1. **Generate local CA** — Go `crypto/x509` + `crypto/ecdsa` (stdlib, no dependencies). Portless shells out to `openssl` but Go stdlib is cleaner.
2. **Per-hostname certs via SNI callback** — generate on demand as worktrees create new hostnames. Memory + disk cache.
3. **`outport trust`** — one-time command to add CA to system trust store:
   - macOS: `security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db ca.pem`
   - Linux: distro-specific (`update-ca-certificates` / `update-ca-trust`)
4. **Cert storage** — `~/.config/outport/certs/`
5. **Renewal** — regenerate 7 days before expiry (Portless uses this approach)

### Portless SSL reference implementation

Source: `github.com/vercel-labs/portless`, `packages/portless/src/certs.ts`

- EC P-256 keys, CA validity 10 years, server certs 1 year
- SNI callback with memory + disk cache + pending-promise dedup
- Byte-peeking to serve TLS and HTTP on same port (first byte 0x16 = TLS handshake)
- HTTP/2 with HTTP/1.1 fallback for WebSocket support
- Rejects SHA-1 signatures, auto-regenerates weak certs

See issue #14 for full details.

## Mobile / Cross-Device Access (2026-03-15)

### `.test` domains cannot work on phones

Investigated all options:

- **Phone DNS config** — technically works but unacceptable UX. Non-starter.
- **mDNS / Bonjour** — only `.local` machine names, not arbitrary app hostnames. Android support inconsistent.
- **iOS `.mobileconfig`** — could set DNS but iOS-only, still a setup step.
- **Conclusion:** No zero-config way to resolve `.test` on a phone. IANA-reserved TLD, no internet DNS resolves it.

### Tunneling is the universal access mechanism

| Scenario | Mechanism |
|----------|-----------|
| Desktop browser | `.test` domains (#13) |
| Phone (simple app, same WiFi) | QR with LAN IP:port (fallback) |
| Phone (complex app) | `outport share` (#16) + QR (#15) |
| Remote colleague | `outport share` (#16) |

- **Simple apps:** QR with `http://<LAN-IP>:<port>` works if the app doesn't reference `localhost` in computed values
- **Multi-service apps:** Computed values contain `localhost` URLs → phone resolves `localhost` to itself → API calls fail. Only tunneling (#16) + env var rewriting (#17) solves this.
- **Security:** Cloudflare Tunnel URLs are high-entropy, temporary, HTTPS, not indexed. Acceptable for dev environments with test data.

### QR code (#15) primary role

Display tunnel URLs from `outport share`, not LAN IPs. LAN IP mode is a fallback for simple apps / no-internet scenarios.

## Competitive Landscape (2026-03-15)

### Layer 6 (multi-service tunnel orchestration) is unserved

No generic, framework-agnostic, local-dev-first tool does automatic env var rewriting when tunneling multiple services. Only:

- **.NET Aspire Dev Tunnels** — `WithReference` injects tunnel URLs between services, but locked to Visual Studio/.NET
- **Preevy** — `PREEVY_BASE_URI_{SERVICE}_{PORT}` in Docker Compose, but deploys to cloud VMs, not local dev

This is Outport's unique value prop. Validated 2026-03-15.

### Closest competitors

| Tool | Layers covered | Key difference from Outport |
|------|---------------|----------------------------|
| **portree** (Rust) | 1,2,3 + partial 6 | Worktree-aware, local `$PT_BACKEND_URL` injection. No tunneling. **Watch this one.** |
| **Portless** (Vercel Labs, TS) | 1,2,3 | Runtime wrapper (injects env into child process, not `.env`). No multi-service wiring. |
| **LocalCan** (macOS GUI) | 2,3,5 | Built-in tunneling + SSL. No port allocation, no worktrees, no orchestration. Commercial. |
| **dot-test** (Go) | 2 + partial 1 | Rails-only, sequential ports, `.test` domains. No worktrees, no computed values. |
| **puma-dev** (Go) | 2,3 | Ruby/Rack only. `.test` domains + SSL. No ports, no worktrees. |
| **Laravel Valet** (PHP) | 2,3,5 | `.test` + SSL + `valet share` (ngrok). PHP-only. |

### Outport's position

Outport is the only tool that:
1. Owns the service map (config file declares all services)
2. Owns the environment files (writes finished `.env` values)
3. Will own the network layer (DNS, proxy, tunnels)

Because it owns all three, it can do multi-service tunnel orchestration (#17) — impossible when these concerns are handled by separate tools.

### Portless detailed analysis

Source cloned to `/Users/steve/src/portless-research`. Full bookmark with discussion notes in Hugo.

**Architecture difference:** Portless is a runtime wrapper (`portless myapp pnpm dev`). Outport is an environment generator (`outport apply`, then use your existing tools). Outport's approach is more durable (`.env` survives restarts, works with Docker Compose) and doesn't require wrapping every command.

**No multi-service wiring** in Portless — each app only gets its own `PORTLESS_URL`. Inter-app communication requires manual framework-specific proxy config. The 508 loop detection is a diagnostic for misconfigured proxies, not orchestration.

**SSL implementation** is a useful reference for #14 (see SSL section above).

## Key Design Decisions (Resolved)

1. **Config file format** — `outport.yml` (YAML) ✅
2. **Port range** — 10000-39999 ✅
3. **Hash input** — `{project}/{instance}/{service}` via FNV-32 ✅
4. **Registry format** — JSON (`~/.config/outport/registry.json`) ✅
5. **Daemon vs on-demand** — on-demand for v1, daemon for v2 (DNS/proxy) ✅
6. **Scope** — v1 ports + .env, v2 DNS/proxy, v3 SSL ✅
7. **Worktree detection** — parse `.git` file for worktree path ✅
8. **Cleanup** — `outport gc` + stale detection in `outport status` ✅

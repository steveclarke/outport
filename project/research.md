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

## SSL / ACME DNS-01

- Let's Encrypt is adding support for DNS-01 challenges via TXT records (2026)
- This would allow real certificates for `.test` domains
- Flow: tool creates DNS TXT record → ACME verifies → issues cert → tool serves HTTPS
- Since we control the DNS server, we can respond to TXT queries automatically
- This would make `https://myapp.test` work with real browser-trusted certificates

## Key Design Decisions to Make

1. **Config file format** — YAML (`.outport.yml`)? TOML? Keep it simple.
2. **Port range** — what range to hash into? 3000-9999? Higher?
3. **Hash input** — project name? Full path? Path + worktree name?
4. **Registry format** — JSON? SQLite? Plain text?
5. **Daemon vs on-demand** — always running (like dot-test) or start/stop per session?
6. **Scope** — v1 just does port allocation + .env writing? DNS/proxy in v2?
7. **Worktree detection** — how to distinguish main checkout from worktrees?
8. **Cleanup** — how/when to release ports from dead worktrees?

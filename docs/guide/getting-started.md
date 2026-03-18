# Getting Started

## Prerequisites

- macOS (Linux support is experimental)
- [Homebrew](https://brew.sh) (recommended) or [Go 1.22+](https://go.dev/dl/)

## Install

```bash
brew install steveclarke/tap/outport
```

See [Installation](/guide/installation) for other methods.

## One-Time System Setup

Run `outport system start` to install the DNS resolver, local CA, and daemon:

```bash
outport system start
```

This sets up `.test` domain resolution, HTTPS certificates, and the reverse proxy. You only need to do this once — the daemon starts at login automatically.

## Create Your Config

Run `outport init` in your project directory:

```bash
cd myapp
outport init
```

This creates `.outport.yml`. Edit it to declare your services:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
```

Each service needs at least an `env_var` — the environment variable that will hold the allocated port.

## Bring It Up

```bash
outport up
```

Output:

```
myapp [main]

    web       PORT        → 24920  https://myapp.test
    postgres  DB_PORT     → 21536
    redis     REDIS_PORT  → 29454
```

Outport allocates deterministic ports for each service and writes them to `.env`:

```bash
# .env
# --- begin outport.dev ---
PORT=24920
DB_PORT=21536
REDIS_PORT=29454
# --- end outport.dev ---
```

Run `outport up` again — you'll get the same ports every time. It's idempotent.

## What Just Happened

When you run `outport up`:

1. **Config loaded** — `.outport.yml` is read from the current directory (or nearest parent).
2. **Instance resolved** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated codes like "bxcf".
3. **Ports allocated** — Each service gets a deterministic port via FNV-32a hash on `"{project}/{instance}/{service}"`. Range: 10000–39999.
4. **Registry updated** — Allocations are saved to `~/.local/share/outport/registry.json`.
5. **.env written** — Ports are written inside a fenced block (`# --- begin/end outport.dev ---`). Your existing `.env` content is preserved.

## Managing the System

```bash
outport system stop        # Stop the daemon
outport system start       # Start the daemon
outport system restart     # Re-write plist and restart the daemon
outport system status      # Show all registered projects
outport system uninstall   # Remove DNS, CA, and daemon entirely
```

## Sharing Services

Need to show your app to someone outside your network, test a webhook, or view it on your phone?

```bash
outport share
```

This tunnels all HTTP services to public Cloudflare URLs and rewrites `.env` files so derived values (CORS, API URLs) automatically point to the tunnel URLs. On exit, everything reverts. Requires `cloudflared` (`brew install cloudflared`). See [Tips & Troubleshooting](/guide/tips#sharing-services-with-outport-share) for details.

## Next Steps

- [Configuration Reference](/reference/configuration) — full `.outport.yml` schema
- [Commands Reference](/reference/commands) — all CLI commands

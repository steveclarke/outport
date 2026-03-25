---
description: Install Outport, run setup, create your first outport.yml config, and allocate deterministic ports for your project.
---

# Getting Started

## Prerequisites

- macOS (Linux support is planned)
- Outport installed — see [Installation](/guide/installation)

## Run Setup

Run the one-time setup:

```bash
outport setup
```

Outport will ask whether to enable `.test` domains with HTTPS. This is recommended — say yes for the full experience (local DNS, reverse proxy, automatic HTTPS) or no to use just the port orchestration.

If you choose yes, you'll be prompted for your password (to configure DNS) and may see a macOS keychain dialog (to trust the local certificate authority). This only happens once.

## Create Your Config

Run `outport init` in your project directory:

```bash
cd myapp
outport init
```

This creates `outport.yml`. Edit it to declare your services:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
computed:
  MYAPP_URL:
    value: "${web.url}"
```

Each service needs at least an `env_var` — the environment variable that will hold the allocated port. The `computed` section wires up values that depend on your services — URLs, CORS origins, API endpoints — so your app gets finished environment variables, not just port numbers.

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
MYAPP_URL=https://myapp.test
# --- end outport.dev ---
```

If you skipped .test domains during setup, the output will show ports without URLs. You can enable them later with `outport system start`.

Run `outport up` again — you'll get the same ports every time. It's idempotent.

## What Just Happened

When you run `outport up`:

1. **Config loaded** — `outport.yml` is read from the current directory (or nearest parent).
2. **Instance resolved** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated codes like "bxcf".
3. **Ports allocated** — Each service gets a deterministic port via FNV-32a hash on `"{project}/{instance}/{service}"`. Range: 10000–39999.
4. **Registry updated** — Allocations are saved to `~/.local/share/outport/registry.json`.
5. **.env written** — Ports and computed values are written inside a fenced block (`# --- begin/end outport.dev ---`). Your existing `.env` content is preserved. Each service can target a different `.env` file — monorepos, sibling directories, even files outside your project — so one `outport up` wires everything.

## Dashboard

Open [https://outport.test](https://outport.test) in your browser for a live dashboard showing all your registered projects, services, and health status. It updates in real-time — when you `outport up` a new project or a service goes up or down, the dashboard reflects it instantly. See [Dashboard](/guide/dashboard) for the full guide.

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

This tunnels all HTTP services to public Cloudflare URLs and rewrites `.env` files so computed values (CORS, API URLs) automatically point to the tunnel URLs. On exit, everything reverts. Requires `cloudflared` (`brew install cloudflared`). See [Sharing & Mobile](/guide/sharing) for details.

## Next Steps

- [Examples](/guide/examples) — real-world configs for monorepos, worktrees, and cross-project setups
- [Configuration Reference](/reference/configuration) — full `outport.yml` schema and template syntax

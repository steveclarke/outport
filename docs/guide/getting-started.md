# Getting Started

## Prerequisites

- macOS (Linux support is experimental)
- [Homebrew](https://brew.sh) (recommended) or [Go 1.22+](https://go.dev/dl/)

## Install

```bash
brew install steveclarke/tap/outport
```

See [Installation](/guide/installation) for other methods.

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

## Apply

```bash
outport apply
```

Output:

```
myapp [main]

    web       PORT        → 24920  http://myapp.test
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

Run `outport apply` again — you'll get the same ports every time. It's idempotent.

## Enable .test Domains (Optional)

For friendly hostnames like `myapp.test` instead of `localhost:24920`:

```bash
outport setup
```

This installs a local DNS server and reverse proxy (requires sudo for the DNS resolver file). Services with `protocol: http` and `hostname` become accessible at their `.test` URL.

Manage the daemon:

```bash
outport up       # Start the daemon
outport down     # Stop the daemon
outport teardown # Remove everything
```

## What Just Happened

When you run `outport apply`:

1. **Config loaded** — `.outport.yml` is read from the current directory (or nearest parent).
2. **Instance resolved** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated codes like "bxcf".
3. **Ports allocated** — Each service gets a deterministic port via FNV-32a hash on `"{project}/{instance}/{service}"`. Range: 10000–39999.
4. **Registry updated** — Allocations are saved to `~/.config/outport/registry.json`.
5. **.env written** — Ports are written inside a fenced block (`# --- begin/end outport.dev ---`). Your existing `.env` content is preserved.

## Next Steps

- [Configuration Reference](/reference/configuration) — full `.outport.yml` schema
- [Commands Reference](/reference/commands) — all CLI commands

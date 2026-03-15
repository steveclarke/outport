---
name: outport
description: Manage dev ports with Outport. Use when setting up a new project, adding services, resolving port conflicts, configuring monorepo cross-service URLs, or working with worktrees. Triggers on "outport", "port conflict", "port allocation", "dev ports", ".outport.yml", "port management", "env var ports", "derived values", "cross-service URLs", "CORS origins from ports". Also use when the user mentions running multiple instances of a project, worktree port setup, or when services need to discover each other's URLs.
---

# Outport — Dev Port Manager

Outport allocates deterministic, non-conflicting ports for dev services and
writes them to `.env` files. Every framework reads `.env` — Rails, Nuxt,
Django, Docker Compose — so ports just work without manual configuration.

## Quick Reference

```bash
outport init              # Create .outport.yml (interactive)
outport apply             # Allocate ports, write .env files
outport a                 # Short alias for apply
outport ports             # Show ports for current project
outport ports --json      # Machine-readable output
outport open              # Open HTTP services in browser
outport open web          # Open a specific service
outport status            # Show all registered projects
outport status --check    # Show with health checks (up/down)
outport apply --force     # Clear and re-allocate all ports
outport unapply           # Remove ports, clean .env files
outport gc                # Remove stale registry entries
```

All commands support `--json` for machine-readable output.

## Core Concept: Ports Live Inside URLs

Outport manages port numbers, but applications don't consume port numbers —
they consume URLs, CORS origins, and connection strings that contain ports.
This is the problem derived values solve.

A frontend doesn't need `RAILS_PORT=24920`. It needs
`NUXT_API_BASE_URL=http://localhost:24920/api/v1`. A backend doesn't need
`MAIN_PORT=21349`. It needs
`CORS_ORIGINS=http://localhost:21349`.

Outport handles both: raw port allocation via `services`, and computed URL
construction via `derived`. The `.outport.yml` config file is the single
source of truth for what Outport manages in your `.env` files.

## Setting Up a New Project

### 1. Create `.outport.yml`

Run `outport init` for interactive setup, or create manually:

```yaml
name: my-project
services:
  web:
    env_var: PORT
    protocol: http
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
```

### 2. Run `outport apply`

Allocates deterministic ports (hashed from project name + service name) and
writes them to `.env`. Same inputs always produce the same ports.

### 3. Wire up your project to read from `.env`

Most frameworks read `.env` natively or with minimal setup:

- **Docker Compose** — reads `.env` automatically. Use `${DB_PORT:-5432}` in
  `compose.yml`
- **Rails** — use `dotenv-rails` gem, or reference env vars in config:
  `port: ENV.fetch("DB_PORT", 5432)`
- **Nuxt** — reads `.env` natively. Runtime config values can be overridden
  via `NUXT_*` env vars
- **Foreman** — reads `.env` automatically
- **Overmind** — does NOT auto-load `.env`. Source it in your start script:
  ```bash
  if [ -f .env ]; then set -a; source .env; set +a; fi
  ```

### 4. Commit `.outport.yml`, gitignore `.env`

`.outport.yml` is project config — commit it so worktrees inherit it.
`.env` contains allocated ports — gitignore it. Each checkout gets its own.

## Config Reference

### Service Fields

| Field | Required | Description |
|-------|----------|-------------|
| `env_var` | yes | Environment variable name written to `.env` |
| `protocol` | no | `http`, `https`, `smtp`, `postgres`, `redis`, etc. HTTP services show URLs in output and work with `outport open` |
| `preferred_port` | no | Port to try first. Falls back to hash-based allocation if taken |
| `hostname` | no | Hostname for URL display. Used by `outport ports`, `outport open`, and JSON output. Defaults to `localhost` |
| `env_file` | no | Where to write. String or array. Defaults to `.env` in project root |

### Writing to Multiple `.env` Files

For monorepos, a port often needs to appear in multiple `.env` files. Use an
array for `env_file`:

```yaml
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file:
      - backend/.env
      - frontend/.env          # Frontend needs this to construct API URLs
```

## Derived Values

Derived values let you define computed env vars that reference allocated
ports. Outport resolves the templates at `outport apply` time and writes
finished values to `.env`.

### Basic syntax

```yaml
derived:
  API_URL:
    value: "http://localhost:${rails.port}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "http://localhost:${web.port}"
    env_file: backend/.env
```

- `${service_name.field}` references service fields (`port`, `hostname`)
- `${service.hostname}` resolves to `localhost` when no hostname is set on the service
- `env_file` is required (no default — you must be explicit)
- Derived names must not collide with service `env_var` names

### Per-file value overrides

When the same env var needs different values in different files (common in
monorepos where multiple apps share a framework convention), use the object
syntax for `env_file` entries:

```yaml
derived:
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/apps/portal/.env
        value: "http://localhost:${rails.port}/portal/api/v1"
```

You can mix string entries (which use a top-level `value`) with object
entries in the same list.

### Real-world monorepo example

This example shows a Rails backend with two Nuxt frontend apps. The backend
needs CORS origins computed from frontend ports. Each frontend needs API and
WebSocket URLs computed from the backend port:

```yaml
name: my-app
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  frontend_main:
    env_var: MAIN_PORT
    protocol: http
    hostname: myapp.localhost
    env_file:
      - frontend/apps/main/.env
      - backend/.env               # Backend needs this for CORS
  frontend_portal:
    env_var: PORTAL_PORT
    protocol: http
    hostname: portal.myapp.localhost
    env_file:
      - frontend/apps/portal/.env
      - backend/.env               # Backend needs this for CORS

derived:
  # Frontend API URLs (different paths per app)
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/apps/portal/.env
        value: "http://localhost:${rails.port}/portal/api/v1"

  # Backend CORS (computed from frontend ports)
  CORE_CORS_ORIGINS:
    value: "http://${frontend_main.hostname}:${frontend_main.port},http://${frontend_portal.hostname}:${frontend_portal.port}"
    env_file: backend/.env

  # Backend asset host
  SHRINE_ASSET_HOST:
    value: "http://${rails.hostname}:${rails.port}"
    env_file: backend/.env
```

After `outport apply`, every service has the right ports AND the right URLs.
No hardcoded values survive.

## Framework Env Var Conventions

When setting up derived values, knowing how frameworks map env vars to
config is essential:

| Framework | Convention | Example |
|-----------|-----------|---------|
| **Nuxt** | `NUXT_` prefix maps to `runtimeConfig` | `NUXT_API_BASE_URL` overrides `runtimeConfig.apiBaseUrl` |
| **Rails (AnyWayConfig)** | `PREFIX_ATTR` maps to config class | `CORE_CORS_ORIGINS` overrides `CoreConfig.cors_origins` |
| **Rails (Shrine)** | Same AnyWayConfig pattern | `SHRINE_ASSET_HOST` overrides `ShrineConfig.asset_host` |
| **Django** | Typically reads `os.environ` directly | Name vars however your `settings.py` expects |
| **Docker Compose** | Reads `.env` automatically | `${DB_PORT:-5432}` in `compose.yml` |

The derived values feature is most powerful when it writes env vars that
match these framework conventions — the framework reads the value natively
and no config code changes are needed.

## .env File Format

Outport writes managed variables in a fenced block at the bottom of each
`.env` file:

```env
# Your own variables — Outport never touches these
SECRET_KEY=abc123
RAILS_ENV=development

# --- begin outport.dev ---
DB_PORT=21536
RAILS_PORT=24920
NUXT_API_BASE_URL=http://localhost:24920/api/v1
# --- end outport.dev ---
```

On each `outport apply`, the fenced block is replaced with current values.
Variables removed from `.outport.yml` disappear from the block. Everything
outside the block is preserved.

## Worktrees

Outport detects git worktrees automatically. Each worktree gets unique
ports — no configuration needed:

```bash
# Main checkout
$ outport apply
my-app [main]
  rails  RAILS_PORT → 24920
  web    MAIN_PORT  → 21349

# Worktree — completely different ports, zero conflicts
$ cd ../my-app-1 && outport apply
my-app [my-app-1 (worktree)]
  rails  RAILS_PORT → 20192
  web    MAIN_PORT  → 21133
```

Derived values are recomputed per worktree — CORS origins, API URLs, and
all other derived values automatically use the worktree's ports.

Two full instances of the same project can run simultaneously with no
port collisions and no manual configuration.

## Integrating with Setup Scripts

Run `outport apply` early in your project's setup flow — after `.env` file
creation but before services start. It should be optional so developers
who haven't installed Outport yet aren't blocked:

```bash
# In bin/setup or similar
if command -v outport > /dev/null 2>&1; then
  outport apply
else
  echo "Outport not found — using default ports"
  echo "Install: brew install steveclarke/tap/outport"
fi
```

## Common Tasks

### Port conflict with another project
Run `outport apply` in both projects. Outport's registry ensures no
collisions across all registered projects.

### Ports are stale from an old allocation
Run `outport apply --force` to clear and re-allocate.

### Freeing ports from a project you're done with
Run `outport unapply` to remove from registry and free all ports.

### Adding a new service to an existing project
Add it to `.outport.yml` and run `outport apply`. Existing allocations
are preserved — only the new service gets a port.

### Agent needs to know the project's URLs
Run `outport ports --json` for structured output with ports, protocols,
and URLs.

### Services moved to different ports than expected
Check `outport status` to see all allocations. If another project holds
the ports you want, unapply it first, then `outport apply --force`.

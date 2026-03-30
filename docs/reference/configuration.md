---
description: Complete outport.yml reference — services, env_var, hostname, preferred_port, env_file, computed values, template syntax, and global settings.
---

# Configuration

Outport has three configuration files:

- **Project config** (`outport.yml`) — lives in each project directory, checked into version control. Declares your services, ports, hostnames, and computed values.
- **Local overrides** (`outport.local.yml`) — optional per-machine overrides, not committed. Merges on top of `outport.yml` at load time.
- **Global settings** (`~/.config/outport/config`) — machine-wide preferences like dashboard health check interval and DNS TTL. Optional — everything has sensible defaults.

## Project Config (`outport.yml`)

### Minimal Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
  postgres:
    env_var: DB_PORT
```

### Typical Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.test
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT

computed:
  API_URL:
    value: "${web.url}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "${frontend.url}"
    env_file: backend/.env
```

### Fields

#### `name` (required)

The project name. Must be unique across all your projects — two projects with the same name will collide in the registry. Used for port allocation, hostname generation, and as the registry key. Lowercase alphanumeric with hyphens.

```yaml
name: my-app
```

#### `services` (required)

A map of service names to their configuration. At least one service is required.

#### `open`

Declares which services `outport open` opens by default. When omitted, `outport open` opens all services with a `hostname`. When present, only the listed services are opened — in the order listed.

```yaml
name: myapp

open:
  - web
  - frontend

services:
  web:
    env_var: PORT
    hostname: myapp.test
  frontend:
    env_var: VITE_PORT
    hostname: app.myapp.test
  admin:
    env_var: ADMIN_PORT
    hostname: admin.myapp.test    # not opened by default
```

Each entry must reference a service that exists and has a `hostname`. You can always open any service explicitly: `outport open admin`.

Can be overridden in `outport.local.yml` — the local list replaces the base list entirely.

#### `env_var` (required)

The environment variable name written to `.env`.

```yaml
services:
  web:
    env_var: PORT
```

#### `hostname`

The `.test` hostname for this service. Must end with `.test`. Implies HTTP — services with a hostname work with `outport open` and get a `.test` domain.

The hostname stem (the part before `.test`) must contain the project name somewhere in it. For a project named `myapp`, valid hostnames include `myapp.test`, `api-myapp.test`, `myapp-admin.test`. This keeps each project's hostnames within its own namespace.

```yaml
# project name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.test
  api:
    env_var: API_PORT
    hostname: api-myapp.test
```

All `.test` hostnames get HTTPS automatically when the local CA is installed — no per-service configuration needed.

For non-main instances, hostnames are automatically suffixed with the instance code: `myapp.test` → `myapp-bkrm.test`.

#### `aliases`

Named alternative hostnames for a service. Each alias registers an additional proxy route to the same port, so a single service can be reached under multiple `.test` hostnames. Keys are short labels used in template variables; values are hostnames that follow the same rules as the primary `hostname` (must contain the project name, globally unique).

```yaml
# project name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis.test
    aliases:
      app: app.approvethis.test
      admin: admin.approvethis.test
```

This exposes `approvethis.test`, `app.approvethis.test`, and `admin.approvethis.test` — all proxied to the same port. For non-main instances, all aliases are suffixed with the instance code just like the primary hostname.

Alias hostnames can be referenced in computed values via `${service.alias.NAME}` and `${service.alias_url.NAME}`.

#### `preferred_port`

Request a specific port. Useful for services like Postgres or MySQL that expect a conventional port. Outport uses this port if it's available. In the rare case another project has already claimed it, Outport falls back to hash-based allocation — you'll see the actual allocated port in the `outport up` output and in your env file.

```yaml
services:
  postgres:
    env_var: DB_PORT
    preferred_port: 5432
```

#### `env_file`

Where to write the environment variable. Defaults to `.env` in the project root. Can be a string or array.

::: warning External paths require approval
Paths outside the project directory (e.g., `../sibling/.env`) require explicit approval. Outport will prompt before writing to external paths. Use `--yes`/`-y` to auto-approve in scripts or CI. Approved paths are remembered so subsequent runs don't re-prompt.
:::

```yaml
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env
  frontend:
    env_var: FRONTEND_PORT
    env_file:
      - frontend/.env
      - shared/.env
```

#### Env File Output

Outport writes managed variables inside a fenced block in your env files:

```bash
# Your existing variables are untouched
DATABASE_URL=postgres://localhost/myapp

# --- begin outport.dev ---
PORT=24920
DB_PORT=21536
REDIS_PORT=29454
# --- end outport.dev ---
```

Everything between the markers is managed by Outport. Everything outside is yours. If you define a variable like `PORT=3000` above the block and Outport also manages `PORT`, it will relocate your definition into the managed block so there's a single source of truth.

#### `computed`

Computed environment variables that reference service values. Useful for wiring URLs between services.

```yaml
computed:
  API_URL:
    value: "${rails.url:direct}/api/v1"
    env_file: frontend/.env
```

#### Template Syntax

Computed values are where Outport goes beyond port allocation. They let you wire services together — build a `DATABASE_URL` from an allocated port, set `CORS_ORIGINS` to another service's URL, or create instance-aware Docker project names that keep worktrees isolated.

Templates use bash-style `${...}` parameter expansion. You can reference any service's port, hostname, or URL, and use operators to handle conditional values like worktree suffixes.

**Service variables:**

| Template | Resolves to | Use case |
|----------|------------|----------|
| `${service.port}` | `24920` | Raw port number |
| `${service.hostname}` | `myapp.test` | `.test` hostname |
| `${service.url}` | `https://myapp.test` | Browser-facing URLs (CORS, asset hosts) |
| `${service.url:direct}` | `http://localhost:24920` | Server-to-server (API calls, WebSocket) |
| `${service.env_var}` | `PORT` | Env var name for the service |
| `${service.alias.NAME}` | `app.myapp.test` | Alias hostname by label |
| `${service.alias_url.NAME}` | `https://app.myapp.test` | Alias URL by label |

Use `${service.url}` for URLs the browser sees — it produces `https://` URLs when the local CA is installed (via `outport system start`). Use `${service.url:direct}` for server-to-server communication that bypasses the proxy (always `http://localhost:{port}`).

**Standalone variables:**

| Template | Resolves to | Use case |
|----------|------------|----------|
| `${instance}` | `""` (main) or `xbjf` (worktree) | Instance-aware values |
| `${project_name}` | `myapp` | Project name from config |

The `${instance}` variable is empty for main instances and set to the instance code for worktrees.

**Operators:**

| Syntax | Meaning | Example |
|--------|---------|---------|
| `${var:-default}` | Use default if var is empty | `${instance:-main}` → `main` for main instances |
| `${var:+replacement}` | Use replacement if var is non-empty | `${instance:+-${instance}}` → `-xbjf` for worktrees, empty for main |

**Real-world example** — unique Docker Compose project names per instance:

```yaml
computed:
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: .env
```

This produces `myapp` for the main instance and `myapp-xbjf` for worktrees, so `docker compose up` from each checkout creates separate container stacks.

#### Per-File Overrides

A computed value's `env_file` can also be an array of objects with `file` and `value` fields. This lets you write different values to different files for the same env var — useful in monorepos where each app needs a different URL or config:

```yaml
computed:
  API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/api/v1"
      - file: frontend/apps/portal/.env
        value: "${rails.url:direct}/portal/api/v1"
```

## Local Overrides (`outport.local.yml`)

For per-machine config that shouldn't be committed to version control, create an `outport.local.yml` in the same directory as your `outport.yml`.

The local file can override any field on services defined in the base config. Fields you specify replace the base values; fields you omit keep their original values.

```yaml
# outport.local.yml (not committed)
services:
  postgres:
    preferred_port: 5432    # use system Postgres on this machine
```

### Rules

- **Override only** — you can only override services that exist in `outport.yml`. Adding new services in the local file produces an error.
- **Field-level merge** — each field you specify replaces the base value. Omitted fields are untouched.
- **Aliases replace entirely** — if you override `aliases`, the entire alias map is replaced, not merged key-by-key.
- **`open` replaces entirely** — if you override `open`, the entire list is replaced.
- **Validation runs on the merged result** — hostname rules, env_var uniqueness, and all other validations apply after merging.
- **No `name` override** — the project name always comes from `outport.yml`.

### Common Uses

| Scenario | Local Override |
|----------|---------------|
| Use system Postgres on port 5432 | `preferred_port: 5432` on the postgres service |
| Write env to a different file on this machine | `env_file: custom/.env` on the service |
| Use a different hostname for local testing | `hostname: dev.myapp.test` on the service |
| Only open specific services on this machine | `open: [web]` at the top level |

### `.gitignore`

Add `outport.local.yml` to your project's `.gitignore`:

```
outport.local.yml
```

## Global Settings

Outport stores machine-level settings in `~/.config/outport/config` (INI format). This file is created by `outport setup` with all values commented out. To change a setting, uncomment the line and edit the value, then run `outport system restart`.

```ini
# Outport global settings
# Uncomment and change values to override defaults.
# Restart the daemon after changes: outport system restart

[dashboard]
# How often the dashboard checks whether services are accepting connections.
# Accepts Go duration syntax: 1s, 5s, 500ms. Minimum 1s.
# health_interval = 3s

[dns]
# Time-to-live in seconds for .test DNS responses. Lower values mean the
# browser picks up service changes faster, but increases DNS queries.
# ttl = 60

[tunnels]
# Maximum number of concurrent tunnel processes when running outport share.
# max = 8

[network]
# Network interface for LAN IP detection (e.g., en0, eth0, wlan0).
# Used by QR codes and the dashboard to show your LAN address.
# When unset, Outport auto-detects by scanning common interface names.
# interface = en0
```

| Setting | Default | Description |
|---------|---------|-------------|
| `dashboard.health_interval` | `3s` | How often the dashboard polls port health. Accepts Go duration syntax (`1s`, `5s`, `500ms`). Minimum `1s`. |
| `dns.ttl` | `60` | Time-to-live (in seconds) for `.test` DNS responses. Lower values mean faster updates when services start/stop. |
| `tunnels.max` | `8` | Maximum number of concurrent tunnel processes when running `outport share`. |
| `network.interface` | _(auto-detect)_ | Network interface for LAN IP detection (e.g., `en0`, `eth0`). Used by QR codes and the dashboard. When unset, Outport scans common interface names. |

Missing settings use defaults. The file is entirely optional — if it doesn't exist, everything uses the defaults above.

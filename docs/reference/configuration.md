---
description: Complete outport.yml reference — services, env_var, hostname, preferred_port, env_file, computed values, and template syntax.
---

# Configuration

Outport is configured with an `outport.yml` file in your project root, checked into version control. It declares your services, how their ports are exposed, and any computed values that wire services together.

## Minimal Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
  postgres:
    env_var: DB_PORT
```

## Typical Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
  postgres:
    env_var: DB_PORT
    preferred_port: 5432
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

## Fields

### `name` (required)

The project name. Used for registry keys, hostname generation, and port hash input. Must be lowercase alphanumeric with hyphens.

```yaml
name: my-app
```

### `services` (required)

A map of service names to their configuration. At least one service is required.

#### `env_var` (required)

The environment variable name written to `.env`.

```yaml
services:
  web:
    env_var: PORT
```

#### `hostname`

The `.test` hostname for this service. Must contain the project name. Implies HTTP — services with a hostname work with `outport open` and get a `.test` domain.

```yaml
services:
  web:
    env_var: PORT
    hostname: myapp
```

This makes the service accessible at `https://myapp.test` (after running `outport system start`). All `.test` hostnames get HTTPS automatically when the local CA is installed — no per-service configuration is needed.

For non-main instances, the hostname is automatically suffixed: `https://myapp-bkrm.test`.

#### `preferred_port`

Request a specific port. Outport uses this port if available, falling back to hash-based allocation if it's taken.

```yaml
services:
  postgres:
    env_var: DB_PORT
    preferred_port: 5432
```

#### `env_file`

Where to write the environment variable. Defaults to `.env` in the project root. Can be a string or array.

Paths outside the project directory (e.g., `../sibling/.env`) require explicit approval. Outport will prompt before writing to external paths. Use `--yes`/`-y` to auto-approve in scripts or CI. Approved paths are remembered so subsequent runs don't re-prompt. Use `--force` to clear saved approvals.

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

### `computed`

Computed environment variables that reference service values. Useful for wiring URLs between services.

```yaml
computed:
  API_URL:
    value: "${rails.url:direct}/api/v1"
    env_file: frontend/.env
```

#### Template Syntax

Computed values use bash-style parameter expansion:

**Service variables:**

| Template | Resolves to | Use case |
|----------|------------|----------|
| `${service.port}` | `24920` | Raw port number |
| `${service.hostname}` | `myapp.test` | `.test` hostname |
| `${service.url}` | `https://myapp.test` | Browser-facing URLs (CORS, asset hosts) |
| `${service.url:direct}` | `http://localhost:24920` | Server-to-server (API calls, WebSocket) |
| `${service.env_var}` | `PORT` | Env var name for the service |

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

When the same env var needs different values per file:

```yaml
computed:
  API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/api/v1"
      - file: frontend/apps/portal/.env
        value: "${rails.url:direct}/portal/api/v1"
```

## .env Output

Outport writes managed variables inside a fenced block:

```bash
# Your existing variables are untouched
DATABASE_URL=postgres://localhost/myapp

# --- begin outport.dev ---
PORT=24920
DB_PORT=21536
REDIS_PORT=29454
# --- end outport.dev ---
```

Variables declared in `outport.yml` are managed by Outport — if they appear outside the fenced block, they're automatically relocated into it.

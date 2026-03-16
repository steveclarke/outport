# Configuration

Outport is configured with a `.outport.yml` file in your project root. This file declares your services and how their ports are exposed.

## Minimal Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
  postgres:
    env_var: DB_PORT
```

## Full Example

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp
  postgres:
    env_var: DB_PORT
    preferred_port: 5432
  redis:
    env_var: REDIS_PORT

derived:
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

#### `protocol`

The service protocol. Services with `http` or `https` protocol work with `outport open` and can have hostnames.

```yaml
services:
  web:
    env_var: PORT
    protocol: http
```

#### `hostname`

The `.test` hostname for this service. Must contain the project name. Only valid for services with `protocol: http` or `protocol: https`.

```yaml
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp
```

This makes the service accessible at `https://myapp.test` (after running `outport setup`). All `.test` hostnames get HTTPS automatically when the local CA is installed — no per-service configuration is needed.

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

### `derived`

Computed environment variables that reference service values. Useful for wiring URLs between services.

```yaml
derived:
  API_URL:
    value: "${rails.url:direct}/api/v1"
    env_file: frontend/.env
```

#### Template Syntax

| Template | Resolves to | Use case |
|----------|------------|----------|
| `${service.port}` | `24920` | Raw port number |
| `${service.hostname}` | `myapp.test` | `.test` hostname |
| `${service.url}` | `https://myapp.test` | Browser-facing URLs (CORS, asset hosts) |
| `${service.url:direct}` | `http://localhost:24920` | Server-to-server (API calls, WebSocket) |

Use `${service.url}` for URLs the browser sees — it produces `https://` URLs when the local CA is installed (via `outport setup`). Use `${service.url:direct}` for server-to-server communication that bypasses the proxy (always `http://localhost:{port}`).

#### Per-File Overrides

When the same env var needs different values per file:

```yaml
derived:
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

Variables declared in `.outport.yml` are managed by Outport — if they appear outside the fenced block, they're automatically relocated into it.

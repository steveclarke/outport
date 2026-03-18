# Examples

Real-world configurations showing how Outport handles common development setups.

## Simple App

A Rails app with Postgres — the minimal useful config:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp.test
  postgres:
    env_var: DB_PORT
    preferred_port: 5432
```

After `outport up`:
- `PORT=23899` and `DB_PORT=5432` written to `.env`
- `https://myapp.test` routes to your Rails server through the proxy

## Monorepo with Multiple Frontends

A monorepo with a Rails API, two Nuxt frontends, Mailpit for email, and Bruno for API testing. Each sub-app has its own `.env` file:

```yaml
name: acme
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    hostname: api.acme.test
    env_file:
      - backend/.env
      - frontend/apps/main/.env
      - frontend/apps/admin/.env
  postgres:
    env_var: DB_PORT
    env_file: backend/.env
  redis:
    env_var: REDIS_PORT
    env_file: backend/.env
  mailpit_web:
    env_var: MAILPIT_WEB_PORT
    protocol: http
    hostname: mailpit.acme.test
    env_file: backend/.env
  mailpit_smtp:
    env_var: MAILPIT_SMTP_PORT
    env_file: backend/.env
  frontend_main:
    env_var: MAIN_PORT
    protocol: http
    hostname: acme.test
    env_file:
      - frontend/apps/main/.env
      - backend/.env
  frontend_admin:
    env_var: ADMIN_PORT
    protocol: http
    hostname: admin.acme.test
    env_file:
      - frontend/apps/admin/.env
      - backend/.env

derived:
  # Frontend API URLs — server-to-server, use :direct
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/api/v1"
      - file: frontend/apps/admin/.env
        value: "${rails.url:direct}/admin/api/v1"

  NUXT_CABLE_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/cable"
      - file: frontend/apps/admin/.env
        value: "${rails.url:direct}/admin/cable"

  # CORS origins — browser-facing, use .test URLs
  APP_CORS_ORIGINS:
    value: "${frontend_main.url},${frontend_admin.url}"
    env_file: backend/.env

  APP_FRONTEND_URL:
    value: "${frontend_main.url}"
    env_file: backend/.env

  # Asset host for file uploads
  ASSET_HOST:
    value: "${rails.url}"
    env_file: backend/.env

  # Docker Compose isolation per worktree
  COMPOSE_PROJECT_NAME:
    value: "acme${instance:+-${instance}}"
    env_file: backend/.env

  # Bruno API testing
  BRUNO_API_URL:
    value: "${rails.url:direct}/api/v1"
    env_file: backend/bruno/.env
  BRUNO_ADMIN_URL:
    value: "${rails.url:direct}/admin/api/v1"
    env_file: backend/bruno/.env
  BRUNO_EXTERNAL_URL:
    value: "${rails.url:direct}/external/api/v1"
    env_file: backend/bruno/.env
```

Key patterns in this config:

- **Per-file overrides** — `NUXT_API_BASE_URL` resolves to different paths depending on which frontend's `.env` it's written to
- **`:direct` vs `.test` URLs** — Server-to-server calls (API, WebSocket) use `${rails.url:direct}` to bypass the proxy. Browser-facing URLs (CORS, asset host) use `${rails.url}` which resolves to the `.test` hostname
- **Shared ports across files** — The Rails port is written to all three `.env` files so each sub-app knows where the API is
- **Bruno integration** — API testing URLs are derived from the same port allocations, staying in sync automatically

## Docker Compose Multi-Instance

When working with git worktrees, each checkout needs its own Docker containers. Without unique project names, `docker compose up` from one worktree replaces the other's containers.

Add a `COMPOSE_PROJECT_NAME` derived value:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp.test
  postgres:
    env_var: DB_PORT

derived:
  COMPOSE_PROJECT_NAME:
    value: "myapp${instance:+-${instance}}"
    env_file: .env
```

The `${instance:+-${instance}}` syntax uses bash-style parameter expansion:
- **Main instance** — `${instance}` is empty, so the result is just `myapp`
- **Worktree** — `${instance}` is `xbjf`, so the result is `myapp-xbjf`

Each checkout gets its own Docker Compose stack with separate containers and volumes.

## Component Library

A Ruby gem with Lookbook (component preview), docs site, and demo app:

```yaml
name: kiso
services:
  lookbook:
    env_var: LOOKBOOK_PORT
    protocol: http
    hostname: lookbook.kiso.test
  docs:
    env_var: DOCS_PORT
    protocol: http
    hostname: docs.kiso.test
  dummy:
    env_var: DUMMY_PORT
    protocol: http
    hostname: kiso.test
```

Three services, each with its own `.test` hostname. No derived values needed since the services don't reference each other.

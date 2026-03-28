---
description: Real-world outport.yml configurations for Rails apps, monorepos, Docker Compose and process-compose multi-instance setups, and cross-project dependencies.
---

# Examples

::: tip Runnable example
Want to see Outport in action before configuring your own project? Clone the [outport-example](https://github.com/steveclarke/outport-example) repo — four Node/Express services wired together with computed values, process-compose orchestration, and a Bruno API collection. Run `bin/dev` and everything starts.
:::

Real-world configurations showing how Outport handles common development setups.

## Simple App

A Rails app with Postgres — the minimal useful config:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.test
  postgres:
    env_var: DB_PORT
```

After `outport up`:

```
myapp [main]

    web       PORT     → 23899  https://myapp.test
    postgres  DB_PORT  → 21536
```

## Monorepo

A Rails API with a Nuxt frontend and Postgres, each with its own `.env` file:

```yaml
name: acme
services:
  rails:
    env_var: RAILS_PORT
    hostname: api.acme.test
    env_file:
      - backend/.env
      - frontend/.env
  frontend:
    env_var: FRONTEND_PORT
    hostname: acme.test
    env_file:
      - frontend/.env
      - backend/.env
  postgres:
    env_var: DB_PORT
    env_file: backend/.env

computed:
  # Server-to-server — use :direct to bypass the proxy
  API_URL:
    value: "${rails.url:direct}/api/v1"
    env_file: frontend/.env

  # Browser-facing — use .test URL for CORS
  CORS_ORIGINS:
    value: "${frontend.url}"
    env_file: backend/.env

  # Docker Compose isolation per worktree
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: backend/.env
```

Key patterns:

- **Per-directory env files** — Each sub-app gets its own `.env`. Ports are shared across files so each service knows where the others are
- **`:direct` vs `.test` URLs** — Server-to-server calls use `${rails.url:direct}` to bypass the proxy. Browser-facing URLs use `${frontend.url}` which resolves to the `.test` hostname
- **Cross-service references** — CORS origins and API URLs are computed from the same port allocations, staying in sync automatically

## Docker Compose Multi-Instance

When working with git worktrees, each checkout may end up with the same Docker Compose project name — especially if your `docker-compose.yml` lives in a subdirectory with a fixed name (like `backend/`). When that happens, `docker compose up` from one worktree replaces the other's containers.

Add a `COMPOSE_PROJECT_NAME` computed value:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.test
  postgres:
    env_var: DB_PORT

computed:
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: .env
```

`${project_name}` resolves to the project name from your config. The `${instance:+-${instance}}` syntax uses bash-style parameter expansion:
- **Main instance** — `${instance}` is empty, so the result is just `myapp`
- **Worktree** — `${instance}` is `xbjf`, so the result is `myapp-xbjf`

Each checkout gets its own Docker Compose stack with separate containers and volumes.

## process-compose Multi-Instance

Similar problem to Docker Compose — [process-compose](https://f1bonacc1.github.io/process-compose/) runs an API server for CLI commands like `process-compose process list` and `process-compose down`. Multiple worktrees collide on the same socket.

process-compose loads a file called `.pc_env` from the current directory at startup — before anything else. Setting `PC_SOCKET_PATH` there automatically enables Unix Domain Socket mode with a unique path per instance:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp.test
  postgres:
    env_var: DB_PORT

computed:
  PC_SOCKET_PATH:
    value: "/tmp/process-compose-${project_name}${instance:+-${instance}}.sock"
    env_file: .pc_env
```

This also shows that `env_file` isn't limited to `.env` — Outport can write computed values to any file. After `outport up`, bare `process-compose` commands (no flags, no wrapper) automatically find the correct socket. Add `.pc_env` to `.gitignore`.

See [Running Your Dev Stack](/guide/devstack#worktree-isolation-with-pc-env) for the full setup including `bin/dev`.

## Cross-Project Dependencies

Outport isn't limited to monorepos. You can write environment variables to any file on your system — even in a completely separate project. This is useful when two independent repos need to know about each other's ports.

For example, a Rails API and a separate Nuxt frontend in different directories:

```yaml
# In ~/src/api/outport.yml
name: myapi
services:
  rails:
    env_var: RAILS_PORT
    hostname: api.myapi.test
    env_file:
      - .env
      - ../frontend/.env
  postgres:
    env_var: DB_PORT

computed:
  API_URL:
    value: "${rails.url}"
    env_file: ../frontend/.env
```

When you run `outport up`, Outport detects that `../frontend/.env` is outside the project directory and asks for approval:

```
⚠ External env files detected:
  ../frontend/.env  →  /Users/you/src/frontend/.env

These files are outside the project directory (/Users/you/src/api).
Allow writing to these files? [y/N]
```

Type `y` and the frontend's `.env` gets the API port and URL. The approval is remembered — subsequent `outport up` runs won't re-prompt. Use `--force` to reset approvals, or `-y` to skip the prompt in scripts and CI.

Every run shows a reminder at the bottom of the output listing which files are written outside the project directory, so you always know what's happening.

This works with absolute paths too:

```yaml
env_file: /Users/you/src/frontend/.env
```

Outport resolves all paths through symlinks before checking boundaries, so tricks like symlinking an external directory into your project won't bypass the approval check.

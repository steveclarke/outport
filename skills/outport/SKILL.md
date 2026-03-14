---
name: outport
description: Manage dev ports with Outport. Use when setting up a new project, adding services, resolving port conflicts, or working with worktrees. Triggers on "outport", "port conflict", "port allocation", "dev ports", ".outport.yml".
---

# Outport — Dev Port Manager

Outport allocates deterministic, non-conflicting ports for dev services and writes them to `.env` files. Use this skill when configuring ports for a project.

## Quick Reference

```bash
outport init          # Create .outport.yml (interactive)
outport up            # Allocate ports, write .env
outport ports         # Show ports for current project
outport ports --json  # Machine-readable output
outport open          # Open HTTP services in browser
outport open web      # Open a specific service
outport status        # Show all registered projects
outport status --check # Show with health checks (up/down)
outport reset         # Clear and re-allocate (tries preferred ports)
outport gc            # Remove stale registry entries
```

## Setting Up a New Project

### 1. Create `.outport.yml`

Run `outport init` for interactive setup, or create manually:

```yaml
name: my-project
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
  postgres:
    preferred_port: 5432
    env_var: DB_PORT
  redis:
    preferred_port: 6379
    env_var: REDIS_PORT
  mailpit_web:
    preferred_port: 8025
    env_var: MAILPIT_WEB_PORT
    protocol: http
  mailpit_smtp:
    preferred_port: 1025
    env_var: MAILPIT_SMTP_PORT
```

### 2. Run `outport up`

This allocates ports and writes them to `.env`. If preferred ports are available, you get them. Otherwise, outport hashes to a deterministic port in the 10000-39999 range.

### 3. Wire up your project to read from `.env`

**Docker Compose** — use env var references:
```yaml
ports:
  - "${DB_PORT:-5432}:5432"
```

**Procfile.dev / Foreman** — reads `.env` automatically:
```
web: bin/rails server -p $PORT
```

**Overmind** — does NOT auto-load `.env`. Add this to `bin/dev`:
```bash
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi
```

**Rails** — use `dotenv-rails` gem or reference env vars in `database.yml`:
```yaml
port: <%= ENV.fetch("DB_PORT", 5432) %>
```

**Nuxt** — reads `.env` natively. Use `NUXT_PORT` or configure in `nuxt.config.ts`.

### 4. Commit `.outport.yml`, gitignore `.env`

`.outport.yml` is project config — commit it so worktrees get it.
`.env` has allocated ports — gitignore it, each checkout gets its own.

## Config Format

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `preferred_port` | yes | Port to try first. Falls back to hash if taken. |
| `env_var` | yes | Environment variable name written to `.env` |
| `protocol` | no | `http`, `https`, `smtp`, `postgres`, `redis`, etc. HTTP services show URLs in output and work with `outport open`. |

### Monorepo with Groups

For projects with multiple env files (e.g., backend + frontend):

```yaml
name: my-monorepo
groups:
  backend:
    env_file: backend/.env
    services:
      rails:
        preferred_port: 3000
        env_var: RAILS_PORT
        protocol: http
      postgres:
        preferred_port: 5432
        env_var: DB_PORT
        env_file:            # override: write to multiple files
          - backend/.env
          - .env
  frontend:
    services:
      web:
        preferred_port: 9000
        env_var: NUXT_PORT
        protocol: http
```

- Groups share an `env_file` — services in the group inherit it
- Per-service `env_file` overrides the group default
- `env_file` can be a string or array (write same var to multiple files)
- Services without a group write to `.env` by default

## Worktrees

Outport detects git worktrees automatically. Each worktree gets unique ports:

- Main checkout tries preferred ports first
- Worktrees get hash-based ports (deterministic per worktree name)
- Run `outport up` in each worktree — no manual port management

## Common Tasks

### Port conflict with another project
Just run `outport up` in both projects. Outport's registry ensures no collisions across all registered projects.

### Ports are stale from an old allocation
Run `outport reset` to clear the current project's allocation and re-allocate fresh, trying preferred ports first.

### Services moved to different ports than expected
Check `outport status` to see all allocations. If preferred ports were taken by another project, outport used hash-based fallback. Either reset the other project first, or accept the hashed ports.

### Adding a new service to an existing project
Add it to `.outport.yml` and run `outport up`. Existing port allocations are preserved — only the new service gets allocated.

### Agent needs to know the project's URLs
Run `outport ports --json` for structured output with ports, protocols, and URLs.

---
name: outport
description: Manage dev ports with Outport. Use when setting up a new project, adding services, resolving port conflicts, or working with worktrees. Triggers on "outport", "port conflict", "port allocation", "dev ports", ".outport.yml".
---

# Outport — Dev Port Manager

Outport allocates deterministic, non-conflicting ports for dev services and writes them to `.env` files. Use this skill when configuring ports for a project.

## Quick Reference

```bash
outport init              # Create .outport.yml (interactive)
outport register          # Register project, allocate ports, write .env
outport reg               # Short alias for register
outport ports             # Show ports for current project
outport ports --json      # Machine-readable output
outport open              # Open HTTP services in browser
outport open web          # Open a specific service
outport status            # Show all registered projects
outport status --check    # Show with health checks (up/down)
outport register --force  # Clear and re-allocate all ports
outport unregister        # Remove from registry, free ports
outport gc                # Remove stale registry entries
```

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
  mailpit_web:
    env_var: MAILPIT_WEB_PORT
    protocol: http
  mailpit_smtp:
    env_var: MAILPIT_SMTP_PORT
```

### 2. Run `outport register`

This allocates deterministic ports and writes them to `.env`. Ports are hashed from the project name, instance, and service name — same inputs always produce the same ports.

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
| `env_var` | yes | Environment variable name written to `.env` |
| `protocol` | no | `http`, `https`, `smtp`, `postgres`, `redis`, etc. HTTP services show URLs in output and work with `outport open`. |
| `preferred_port` | no | Port to try first. Falls back to hash if taken. Omit to let Outport assign a deterministic port automatically. |

### Per-Service `env_file`

For projects with multiple env files (e.g., backend + frontend), use per-service `env_file`:

```yaml
name: my-monorepo
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  postgres:
    env_var: DB_PORT
    env_file: backend/.env
  web:
    env_var: NUXT_PORT
    protocol: http
```

- Services without `env_file` write to `.env` by default
- `env_file` can be a string or array (write same var to multiple files)

## Worktrees

Outport detects git worktrees automatically. Each worktree gets unique ports:

- Main checkout and worktrees all get deterministic hash-based ports
- Run `outport register` in each worktree — no manual port management

## Common Tasks

### Port conflict with another project
Just run `outport register` in both projects. Outport's registry ensures no collisions across all registered projects.

### Ports are stale from an old allocation
Run `outport register --force` to clear the current project's allocation and re-allocate fresh.

### Freeing ports from a project you're done with
Run `outport unregister` to remove the project from the registry and free all its ports.

### Services moved to different ports than expected
Check `outport status` to see all allocations. If another project has the ports you want, unregister that project first, then `outport register --force` in yours.

### Adding a new service to an existing project
Add it to `.outport.yml` and run `outport register`. Existing port allocations are preserved — only the new service gets allocated.

### Agent needs to know the project's URLs
Run `outport ports --json` for structured output with ports, protocols, and URLs.

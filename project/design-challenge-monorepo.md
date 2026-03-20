# Design Challenge: Monorepo & Complex Service Architectures

## The Problem

The current `outport.yml` design assumes a simple model: one project, one `.env` file, ports are the only moving part. Real-world projects like Unio break this in several ways.

## Case Study: Unio

Unio is a monorepo with 7 services across 3 tiers:

| Service | Framework | Port | Env File Location | How Port Is Configured |
|---------|-----------|------|-------------------|----------------------|
| Rails API | Rails 8.1 | 3000 | `backend/.env` | `RAILS_PORT` in `.env` |
| Main App | Nuxt 4.3 | 9000 | hardcoded | `package.json` script / nuxt.config |
| Portal App | Nuxt 4.3 | 9001 | hardcoded | `package.json` script / nuxt.config |
| PostgreSQL | Postgres 18 | 5432 | `backend/.env` | `DB_PORT` in `.env` |
| Redis | Redis 7 | 6379 | `backend/.env` | `REDIS_PORT` in `.env` |
| Mailpit Web | Mailpit | 8025 | `backend/.env` | `MAILPIT_WEB_PORT` in `.env` |
| Mailpit SMTP | Mailpit | 1025 | `backend/.env` | `MAILPIT_SMTP_PORT` in `.env` |

### Challenge 1: Hostname + Port Coupling

The frontend apps don't just need ports — they need specific hostnames:
- Main app: `unio.localhost:9000`
- Portal app: `portal.unio.localhost:9001`

These hostnames appear in:
- **CORS config** (`backend/config/core.yml`) — the API must allow requests from these origins
- **Cookie configuration** — separate domains for separate cookie jars
- **Nuxt config** — the dev server binds to these hostnames

When the port changes, every reference to `unio.localhost:9000` must also change. Outport currently has no concept of hostnames.

### Challenge 2: Multiple Env File Locations

The `.env` file isn't at the project root — it's at `backend/.env`. In a monorepo:
- Backend services read from `backend/.env`
- Frontend apps might read from `frontend/.env` or not use `.env` at all
- Root-level `.env` might exist for Docker Compose or the process manager

A single `env_file: .env` doesn't cut it. Each service might write to a different file.

### Challenge 3: Non-Env Port Configuration

Not everything reads from `.env`:
- Nuxt ports are set in `package.json` scripts or `nuxt.config.ts`
- Some frameworks use TOML, YAML, or JSON config files
- Docker Compose reads `.env` but the port mapping syntax is specific

Writing to `.env` covers 80% of cases. The other 20% need a different integration mechanism.

### Challenge 4: CORS and URL References

When ports change, CORS allowlists must match. Unio's `core.yml`:

```yaml
development:
  cors_origins:
    - http://unio.localhost:9000
    - http://portal.unio.localhost:9001
```

This isn't an env var — it's a YAML config file with full URLs. Outport would need to either:
- Template these files
- Write additional env vars that the config file reads
- Stay out of it and let the user handle CORS manually

## Questions to Resolve

### Per-service env file paths

Should each service declare where its env file lives?

```yaml
services:
  rails:
    default_port: 3000
    env_var: RAILS_PORT
    env_file: backend/.env
  postgres:
    default_port: 5432
    env_var: DB_PORT
    env_file: backend/.env
  main:
    default_port: 9000
    env_var: NUXT_PORT
    env_file: frontend/apps/main/.env
```

Or should there be a top-level default with per-service overrides?

```yaml
env_file: backend/.env

services:
  rails:
    default_port: 3000
    env_var: RAILS_PORT
  main:
    default_port: 9000
    env_var: NUXT_PORT
    env_file: frontend/apps/main/.env  # override
```

### Hostnames as a first-class concept

Should Outport manage hostnames now (v1), or defer to the reverse proxy (v2)?

If v1: the config declares a hostname and Outport writes it to `.env` alongside the port:

```yaml
services:
  main:
    default_port: 9000
    env_var: MAIN_PORT
    protocol: http
    hostname: unio.localhost
    env:
      MAIN_URL: "http://{hostname}:{port}"
```

If v2: hostnames are only relevant when the proxy is running. Simpler v1, but CORS config is still a manual problem.

### Template files (adapters)

Agent 1 mentioned this in the original brainstorm — adapters for different file types. Should Outport be able to template arbitrary config files?

```yaml
templates:
  - source: backend/config/core.yml.template
    target: backend/config/core.yml
  - source: frontend/apps/main/nuxt.config.template.ts
    target: frontend/apps/main/nuxt.config.ts
```

This is powerful but dramatically increases scope. It turns Outport from a port allocator into a config templating engine.

Alternative: Outport writes env vars, and the config files read env vars. Push the integration burden onto the project, not the tool.

### Where does Outport draw the line?

The fundamental tension:

- **Minimal:** Outport allocates ports and writes `.env`. Everything else is the project's problem. Simple, portable, easy to understand.
- **Maximal:** Outport manages ports, hostnames, URLs, CORS, connection strings, config files. Powerful but complex, opinionated, hard to maintain.

The right answer is probably: **Outport allocates ports and writes env vars to configurable file locations.** Projects that read from env vars get it for free. Projects with hardcoded config need to be adapted to read env vars — that's a one-time migration, not an Outport problem.

## Proposed Minimum Viable Config for Unio

```yaml
name: unio

env_file: backend/.env  # default for all services

services:
  rails:
    default_port: 3000
    env_var: RAILS_PORT
    protocol: http

  main:
    default_port: 9000
    env_var: MAIN_PORT
    protocol: http
    env_file: .env  # root .env, read by Procfile/Overmind

  portal:
    default_port: 9001
    env_var: PORTAL_PORT
    protocol: http
    env_file: .env

  postgres:
    default_port: 5432
    env_var: DB_PORT
    protocol: postgres

  redis:
    default_port: 6379
    env_var: REDIS_PORT
    protocol: redis

  mailpit_web:
    default_port: 8025
    env_var: MAILPIT_WEB_PORT
    protocol: http

  mailpit_smtp:
    default_port: 1025
    env_var: MAILPIT_SMTP_PORT
    protocol: smtp
```

This doesn't solve CORS or hostname routing. But it solves port allocation and multi-file env writing, which is the 80% case.

## Existing Solution in Unio

Unio already has `bin/ports` which does index-based port bumping:
- `bin/ports --index 1` offsets all ports by a fixed amount
- Ports are deterministic per index
- Manual but functional

Outport should be strictly better than this — deterministic without manual indexing, works across projects not just within one.

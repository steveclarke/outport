# Configuration

> Config format, fields, rules, and edge cases for `.outport.yml`.

## Status: Draft

This spec captures design thinking from early conversations. It is NOT finalized. The current implementation may differ — this document describes where the design is heading.

## Core Principle

The developer should not have to think about port numbers. The default experience is: declare your services, run `outport register`, get deterministic ports. Done.

Port numbers are an implementation detail that Outport abstracts away — the same way DNS abstracts away IP addresses.

## Config File

### `.outport.yml` (committed to repo)

The project config declares what services exist and how to expose them via environment variables. It does NOT (by default) specify port numbers.

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
  postgres:
    env_var: DATABASE_PORT
  redis:
    env_var: REDIS_PORT
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Project name. Used in registry keys, hostname generation, and hash input. |
| `services` | Yes | Map of service name → service config. At least one required. |
| `env_var` | Yes | Environment variable name written to `.env`. |
| `protocol` | No | `http`, `https`, `smtp`, `postgres`, `redis`, etc. HTTP services get URLs in output and work with `outport open`. |
| `env_file` | No | Where to write the env var. Defaults to `.env`. Can be a string or array for multi-file writes. |
| `fixed_port` | No | Lock this service to a specific port. See "Fixed Ports" below. |

### Multiple .env files (monorepo support)

Use per-service `env_file` to write to different `.env` files:

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

- `env_file` defaults to `.env` in the project root if omitted
- Can be a string or array (write same var to multiple files)

## Port Assignment (Default Behavior)

When no `fixed_port` is specified, Outport assigns a deterministic port via FNV-32a hash on `{project}/{instance}/{service}`. The hash maps to the range 10000–39999.

- **Deterministic**: same project + worktree + service = same port, every time
- **Order-independent**: doesn't matter which project runs `outport register` first
- **Collision-free**: linear probing resolves hash collisions across all registered projects

This is the primary and recommended mode. Most developers should never specify a port number.

## Fixed Ports

`fixed_port` locks a service to a specific port number. This is the exception, not the default.

```yaml
services:
  postgres:
    env_var: DATABASE_PORT
    fixed_port: 5432    # always this port, error if taken
```

### When to use fixed ports

- You're using a system-installed service (e.g., system Postgres on 5432) rather than a Docker container
- An external tool or integration expects a specific port that can't be configured via env var
- You have bookmarks or muscle memory for a specific port and want to keep it

### When NOT to use fixed ports

- "Because Rails defaults to 3000" — Outport replaces that default
- "Because I want nice round numbers" — hostnames replace port numbers entirely
- For most services in most projects

### Fixed ports and worktrees

There are two distinct scenarios for why someone fixes a port:

**Scenario 1: "I prefer this port, but worktrees can do their own thing"**

Main checkout gets Postgres on 5432 because it's convenient. But worktrees spin up their own Dockerized Postgres and don't care what port they get. The fixed port is a preference tied to the main instance.

```yaml
services:
  postgres:
    env_var: DATABASE_PORT
    fixed_port: 5432          # main gets 5432, worktrees get dynamic ports
```

**Scenario 2: "This port, always, every instance shares it"**

All instances — main and every worktree — use the same system Postgres on 5432. The port is fixed to the infrastructure, not to a project instance. Worktrees don't get their own database server.

```yaml
services:
  postgres:
    env_var: DATABASE_PORT
    fixed_port: 5432
    shared: true              # every instance uses this same port
```

**Design decision:**

`fixed_port` without `shared` applies to main only. Worktrees get dynamic (hash-based) ports. This is the default because it's the common case — worktrees are isolated.

`fixed_port` with `shared: true` applies to every instance. Use this when worktrees share infrastructure (e.g., a single system-installed database).

The two axes:

| | Isolated (own instance per worktree) | Shared (all worktrees use same instance) |
|---|---|---|
| **No fixed_port** | Everyone gets dynamic ports (default) | _(use shared without fixed_port? TBD)_ |
| **fixed_port** | Main gets fixed, worktrees get dynamic | Every instance gets the fixed port |

### Terminology

- **Dynamic port** — assigned by Outport via deterministic hash. The default.
- **Fixed port** — locked to a specific number. Opt-in override.
- **Shared** — all instances (main + worktrees) use the same port/service. Modifier on fixed_port.
- **Isolated** — each instance gets its own port. The default.

This needs further validation through real-world setup before finalizing.

## Services You Don't Put in the Config

If a service's port is managed outside of Outport (e.g., a system service you always run on its default port), you can simply leave it out of `.outport.yml`. Outport only manages what you declare.

The tradeoff: leaving it out means `.outport.yml` isn't a complete picture of your project's services. Including it with `fixed_port` serves as documentation — "this project uses Postgres on 5432" — even if Outport isn't dynamically assigning the port.

This is a developer preference. Both approaches are valid.

## `.outport.local.yml` (not committed)

_(Design question: do we need a local override file?)_

Use case: the committed `.outport.yml` declares services generically, but on your specific machine or in a specific worktree, you want to override something — lock a port, change an env file path, etc.

Pattern is familiar from `.env` / `.env.local` in Rails.

```yaml
# .outport.local.yml (gitignored)
services:
  postgres:
    fixed_port: 5432    # on this machine, use system Postgres
```

This would be merged on top of `.outport.yml` at load time. Not yet decided whether this is needed — the `fixed_port` field in the main config might be sufficient for most cases.

## CLI Commands

Commands should say what they do. No arcane abbreviations.

### `outport init`

Creates `.outport.yml`. One-time setup. Does NOT register or allocate ports.

Interactive setup should:

1. Ask for the project name (default: directory name)
2. Ask which services to include (from presets: web, postgres, redis, mailpit, vite, etc.)
3. NOT ask about port numbers — that's the whole point
4. Generate a clean `.outport.yml` with just names, env vars, and protocols
5. Optionally add `env_file` if the user specifies a non-default location

### `outport register`

Reads `.outport.yml`, allocates ports, saves to the central registry, writes `.env`. This is the command that actually does the work. Alias: `outport reg`.

- Idempotent — running again reuses existing allocations
- `--force` flag to re-allocate fresh (replaces the current `outport reset` command)
- Does NOT start any services or daemons

### `outport unregister`

Removes the project/instance from the central registry, freeing its ports. Replaces the concept from issue #12 (`outport down`).

- Does NOT stop any running services

### Future commands

- `outport up` / `outport down` — reserved for starting/stopping the DNS + proxy daemon (not port allocation)
- `outport share` — tunneling

## Validation Rules

- `name` is required
- At least one service is required
- Every service must have `env_var`
- `env_var` must be unique per target `env_file`
- `fixed_port` must be a valid port number if specified

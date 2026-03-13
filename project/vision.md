# Outport — Dev Port Manager

## The Problem

Modern development means running multiple projects simultaneously on one machine — Rails apps, Nuxt frontends, Docker services (Postgres, Redis, Mailpit). Each needs its own ports. With agentic coding (Claude Code, Codex) and git worktrees, you might run 3-5 instances of the *same* project at once.

Port conflicts are constant. You start a second project and Postgres is already on 5432. You spin up a worktree and Rails is already on 3000. You waste time figuring out which port to use, updating configs, and remembering what's running where.

The only thing holding back parallel worktree development is how cumbersome it is to set up ports for every instance.

## The Vision

**Outport makes ports disappear.** You declare what services your project needs, and Outport handles the rest — allocates ports, writes them to your `.env`, gives you clean URLs like `myapp.test`, and optionally handles local SSL.

Drop a config file in any project. Run `outport up`. Done.

## What It Does

### 1. Port Allocation
- Each project/worktree gets deterministic, non-conflicting ports for all its services
- Ports are allocated from a central registry (`~/.config/outport/`)
- Works across different projects (Rails, Nuxt, anything) and across worktrees of the same project
- Writes allocated ports to the project's `.env` file (the universal integration point)

### 2. Service Discovery
- Projects declare their services in a config file (e.g., `.outport.yml`)
- Each service has a default port and a name
- Outport allocates actual ports and makes them available via `.env` variables

### 3. Nice URLs (via DNS + Reverse Proxy)
- `outport-dev.test` instead of `localhost:3100`
- `outport-dev-feature-xyz.test` for worktrees
- Built-in DNS server + HTTP reverse proxy (inspired by dot-test)
- Automatic subdomain routing for worktrees

### 4. Local SSL (future)
- ACME DNS-01 challenge support (Let's Encrypt, coming 2026)
- Real certificates for `.test` domains via DNS TXT records
- `https://myapp.test` that just works

## Per-Project Config

```yaml
# .outport.yml
name: outport-dev
services:
  web:
    default_port: 3000
    env_var: PORT
  postgres:
    default_port: 5432
    env_var: DATABASE_PORT
  mailpit_web:
    default_port: 8025
    env_var: MAILPIT_WEB_PORT
  mailpit_smtp:
    default_port: 1025
    env_var: MAILPIT_SMTP_PORT
```

## Central Registry

```
~/.config/outport/
  registry.json       # All known projects and their allocated ports
```

## Integration Points

- **`.env` files** — primary output. Docker Compose, Foreman, Nuxt, Rails all read `.env`
- **Docker Compose** — `compose.yml` reads port vars from `.env` (e.g., `${DATABASE_PORT:-5432}`)
- **Procfile.dev** — `web: bin/rails server -p $PORT`
- **Framework-agnostic** — any project that reads environment variables works

## CLI

```
outport init          # Create .outport.yml for this project (interactive)
outport up            # Allocate ports, write .env, start DNS/proxy
outport down          # Stop DNS/proxy, release ports
outport status        # Show all registered projects and their ports
outport ports         # Show ports for current project
outport sync          # Re-scan and update port allocations
```

## Design Principles

1. **Zero thinking** — `outport up` and forget about ports forever
2. **Convention over configuration** — sensible defaults, config only when needed
3. **`.env` is the contract** — that's how ports get to your tools
4. **Framework-agnostic** — Rails, Nuxt, Phoenix, Django, anything
5. **Worktree-native** — first-class support for parallel worktree development
6. **Single binary** — Go, no dependencies, install via Homebrew

## Prior Art

- **dot-test** (Obi Fernandez) — DNS + reverse proxy for `.test` domains, Rails-only, no backing service awareness
- **Kiso bin/worktree** — MD5 hash-based port allocation for worktrees, project-specific
- **Portree** — FNV32 hash ports, TUI dashboard, no Docker awareness
- **Portless** (Vercel) — random ports + proxy, worktree-aware subdomains
- **worktree-compose** — Docker Compose isolation per worktree, port formula
- **devports** — template-based config + port registry, mentions LLM agent use cases
- **acquire-port** — deterministic port from project name (npm)

None of these solve the full problem: multiple different projects + worktrees + backing services + nice URLs + SSL.

## Tech Stack

- **Go** — single binary, no runtime dependencies
- **Homebrew** — primary distribution
- **Domain:** outport.app

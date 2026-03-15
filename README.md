<p align="center">
  <a href="https://outport.app">
    <img src="brand/svg/logo-horizontal-color.svg" width="280" alt="Outport">
  </a>
</p>

# Outport

Dev port manager for multi-project, multi-worktree development.

Outport allocates deterministic, non-conflicting ports for all your projects and writes them to `.env` files. No more port conflicts. No more guessing what's running where.

## The Problem

You're running Rails on 3000, Nuxt on 5173, Postgres on 5432. You start a second project — port conflict. You spin up a git worktree — another conflict. With agentic coding tools and parallel development, you might run 3-5 instances of the same project simultaneously.

Outport fixes this. Declare your services once, run `outport apply`, and never think about ports again.

## Quick Start

```bash
# In your project directory
outport init          # Create .outport.yml (interactive)
outport apply      # Allocate ports, write .env
```

That's it. Your `.env` now has deterministic, non-conflicting ports:

```
DATABASE_PORT=39972
PORT=39519
REDIS_PORT=30938
```

## How It Works

Drop a `.outport.yml` in your project:

```yaml
name: myapp
services:
  web:
    env_var: PORT
  postgres:
    env_var: DATABASE_PORT
  redis:
    env_var: REDIS_PORT
```

Run `outport apply`. Outport allocates a deterministic hash-based port (range 10000-39999) for each service and writes the result to `.env`.

### Preferred Ports (optional)

You can hint at a preferred port for any service. If it's available, you get it — otherwise Outport falls back to hash-based allocation:

```yaml
services:
  web:
    env_var: PORT
    preferred_port: 3000
  postgres:
    env_var: DATABASE_PORT
    preferred_port: 5432
```

Same project, same worktree, same ports. Every time.

### Worktree Support

Outport detects git worktrees automatically. Each worktree gets its own set of ports:

```bash
# Main checkout
$ outport apply
outport: myapp
  web (PORT) → 39519
  postgres (DATABASE_PORT) → 39972

# Feature worktree — completely different ports, zero conflicts
$ outport apply
outport: myapp [feature-xyz (worktree)]
  web (PORT) → 28104
  postgres (DATABASE_PORT) → 13567
```

## Integration

Outport writes to `.env` because everything already reads it:

- **Docker Compose** reads `.env` automatically — use `${DATABASE_PORT:-5432}` in `compose.yml`
- **Foreman / Overmind** reads `.env` — use `web: bin/rails server -p $PORT` in `Procfile.dev`
- **Rails** (dotenv-rails), **Nuxt**, **Phoenix**, **Django** — all have dotenv support
- **Any framework** that reads environment variables works with zero configuration

Outport preserves your existing `.env` variables. It only updates variables declared in your `.outport.yml` — everything else is preserved.

## Commands

```
outport init              Create .outport.yml for this project
outport apply          Register project, allocate ports, write .env
outport a                 Short alias for apply
outport apply --force  Clear and re-allocate all ports
outport unregister        Remove from registry, free ports
outport ports             Show ports for the current project
outport open              Open HTTP services in the browser
outport status            Show all registered projects
outport status --check    Show with health checks (up/down)
outport gc                Remove stale registry entries
```

All commands support `--json` for machine-readable output.

## How Ports Are Allocated

Outport uses FNV-32 hashing on `{project}/{instance}/{service}` to produce a deterministic port in the 10000-39999 range. If you specify a `preferred_port` and it's available, you get it — so your main checkout can keep familiar ports like 3000 and 5432. Allocations are persisted in `~/.config/outport/registry.json`.

Ports are stable: once allocated, running `outport apply` again reuses the same ports. New services added to your config get fresh allocations without disturbing existing ones.

## Protocol

Add `protocol` to services to get URLs in output and enable `outport open`:

```yaml
services:
  web:
    env_var: PORT
    protocol: http       # shows http://localhost:<port> in output
  postgres:
    env_var: DB_PORT     # no protocol — just shows port number
```

Supported protocols: `http`, `https`, `smtp`, `postgres`, `redis`, and any custom string.

## Multiple .env Files

Use per-service `env_file` to write ports to different locations — useful for monorepos:

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

Services without `env_file` write to `.env` in the project root. `env_file` can be a string or array to write to multiple files.

## AI Agent Skill

Install the outport skill so your AI coding agent knows how to configure ports:

```bash
npx skills add steveclarke/outport/skills
```

## Install

### Homebrew

```bash
brew install steveclarke/tap/outport
```

### From Source

```bash
go install github.com/steveclarke/outport@latest
```

### Build Locally

```bash
git clone https://github.com/steveclarke/outport.git
cd outport
go build -o outport .
```

## Development

Requires [Go 1.24+](https://go.dev/dl/) and [just](https://github.com/casey/just).

```bash
just build        # Build the binary
just test         # Run all tests
just lint         # Run linter
just run apply     # Build and run with args
just clean        # Clean build artifacts
```

## Roadmap

- **v1 (current):** Port allocation + apply/unregister + `.env` writing
- **v2:** DNS server + reverse proxy for `.test` domains (`myapp.test` instead of `localhost:39519`)
- **v3:** Local SSL with real certificates for `.test` domains

## License

MIT

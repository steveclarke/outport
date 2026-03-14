# Outport

Dev port manager for multi-project, multi-worktree development.

Outport allocates deterministic, non-conflicting ports for all your projects and writes them to `.env` files. No more port conflicts. No more guessing what's running where.

## The Problem

You're running Rails on 3000, Nuxt on 5173, Postgres on 5432. You start a second project — port conflict. You spin up a git worktree — another conflict. With agentic coding tools and parallel development, you might run 3-5 instances of the same project simultaneously.

Outport fixes this. Declare your services once, run `outport up`, and never think about ports again.

## Quick Start

```bash
# In your project directory
outport init          # Create .outport.yml (interactive)
outport up            # Allocate ports, write .env
```

That's it. Your `.env` now has deterministic, non-conflicting ports:

```
DATABASE_PORT=39972 # managed by outport
PORT=39519 # managed by outport
REDIS_PORT=30938 # managed by outport
```

## How It Works

Drop a `.outport.yml` in your project:

```yaml
name: myapp
services:
  web:
    default_port: 3000
    env_var: PORT
  postgres:
    default_port: 5432
    env_var: DATABASE_PORT
  redis:
    default_port: 6379
    env_var: REDIS_PORT
```

Run `outport up`. Outport hashes your project name + worktree + service name into a deterministic port (range 10000-39999), checks for collisions with other registered projects, and writes the result to `.env`.

Same project, same worktree, same ports. Every time.

### Worktree Support

Outport detects git worktrees automatically. Each worktree gets its own set of ports:

```bash
# Main checkout
$ outport up
outport: myapp [main]
  web (PORT) → 39519
  postgres (DATABASE_PORT) → 39972

# Feature worktree — completely different ports, zero conflicts
$ outport up
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

Outport preserves your existing `.env` variables. It only manages lines marked with `# managed by outport`.

## Commands

```
outport init      Create .outport.yml for this project (interactive)
outport up        Allocate ports and write to .env
outport ports     Show ports for the current project
outport status    Show all registered projects and their ports
outport gc        Remove stale entries from the registry
```

## How Ports Are Allocated

Outport uses FNV-32 hashing on `{project}/{instance}/{service}` to produce deterministic ports. If two projects hash to the same port (rare), linear probing finds the next available one. Allocations are persisted in `~/.config/outport/registry.json`.

Ports are stable: once allocated, running `outport up` again reuses the same ports. New services added to your config get fresh allocations without disturbing existing ones.

## Install

### From Source

```bash
go install github.com/outport-app/outport@latest
```

### Build Locally

```bash
git clone https://github.com/outport-app/outport.git
cd outport
go build -o outport .
```

## Development

Requires [Go 1.24+](https://go.dev/dl/) and [just](https://github.com/casey/just).

```bash
just build        # Build the binary
just test         # Run all tests
just lint         # Run linter
just run up       # Build and run with args
just clean        # Clean build artifacts
```

## Roadmap

- **v1 (current):** Port allocation + `.env` writing
- **v2:** DNS server + reverse proxy for `.test` domains (`myapp.test` instead of `localhost:39519`)
- **v3:** Local SSL with real certificates for `.test` domains

## License

MIT

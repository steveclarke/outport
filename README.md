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
# --- begin outport.dev ---
DATABASE_PORT=39972
PORT=39519
REDIS_PORT=30938
# --- end outport.dev ---
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
outport unapply           Remove ports, clean .env files
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

## Protocol and Hostname

Add `protocol` to services to get URLs in output and enable `outport open`. Add `hostname` to control the hostname in URLs (defaults to `localhost`):

```yaml
services:
  web:
    env_var: PORT
    protocol: http                # shows URL in output, enables 'outport open'
    hostname: myapp.localhost     # optional — defaults to localhost
  postgres:
    env_var: DB_PORT              # no protocol — just shows port number
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

## Derived Values

Applications don't just need port numbers — they need URLs. Derived values let you define computed environment variables that reference allocated ports:

```yaml
name: my-monorepo
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  web:
    env_var: WEB_PORT
    protocol: http
    env_file: frontend/.env

derived:
  API_URL:
    value: "http://localhost:${rails.port}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "http://localhost:${web.port}"
    env_file: backend/.env
```

After `outport apply`, `frontend/.env` contains:

```
# --- begin outport.dev ---
API_URL=http://localhost:24920/api/v1
WEB_PORT=14139
# --- end outport.dev ---
```

Templates use `${service_name.field}` syntax — reference any service's `port` or `hostname`. Resolved at apply time — your app reads finished values from `.env`.

When the same env var needs different values per file (common in monorepos), use per-file overrides:

```yaml
derived:
  NUXT_API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/apps/portal/.env
        value: "http://localhost:${rails.port}/portal/api/v1"
```

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

## FAQ

### "My frontend and backend need to know each other's URLs, not just ports"

Use [derived values](#derived-values). Outport computes URLs from allocated ports and writes finished env vars to `.env`.

### "I'm running two worktrees and my sessions are colliding"

Browsers share cookies across ports on the same hostname. If both worktrees serve on `localhost`, they share a cookie jar. Workaround: use an incognito window or a separate browser profile for the second worktree. Long-term: `.test` domain support ([#13](https://github.com/steveclarke/outport/issues/13)) will give each worktree its own hostname.

### "I have a monorepo where two apps use the same env var name but need different values"

Use [per-file value overrides](#derived-values) in derived values:

```yaml
derived:
  API_BASE_URL:
    env_file:
      - file: frontend/app-a/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/app-b/.env
        value: "http://localhost:${rails.port}/admin/api/v1"
```

### "How do I add Outport to my project's setup script?"

Make it optional so developers without Outport aren't blocked:

```bash
if command -v outport > /dev/null 2>&1; then
  outport apply
else
  echo "Outport not found — install: brew install steveclarke/tap/outport"
fi
```

### "Can AI coding agents use Outport?"

Yes. Install the Outport skill so your agent knows the commands and patterns:

```bash
npx skills add steveclarke/outport/skills
```

The agent can run `outport apply` in worktrees, read `outport ports --json` for structured output, and configure `.outport.yml` for new services.

## How Outport Compares

Most local dev tools solve one piece of the puzzle — naming ports, or providing SSL, or tunneling. Outport takes a different approach: it owns the full service map and writes finished, computed environment variables to `.env`. This matters most when your project has multiple services that need to discover each other, or when you're running parallel worktrees with AI agents.

This isn't about competition — these are all good tools. This grid helps you see if Outport fits your workflow.

| | Outport | [Portless](https://github.com/vercel-labs/portless) | [portree](https://github.com/fairy-pitta/portree) | [dot-test](https://github.com/zarpay/dot-test) | [puma-dev](https://github.com/puma/puma-dev) | [Laravel Valet](https://laravel.com/docs/valet) |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Deterministic ports** | Yes (hash) | Ephemeral | Yes (hash) | Sequential | No | No |
| **Worktree-aware** | Yes | Yes | Yes | No | No | No |
| **Multi-service wiring** | Yes | No | Partial | No | No | No |
| **Writes to .env** | Yes | No¹ | No | Yes² | No | No |
| **Friendly hostnames** | Planned | Yes | Yes | Yes | Yes | Yes |
| **SSL certificates** | Planned | Yes | Yes | No | Yes | Yes |
| **Framework-agnostic** | Yes | Yes | Yes | Rails only | Ruby/Rack | PHP/Laravel |
| **No runtime wrapper** | Yes³ | No | No | Yes³ | No | No |
| **Single binary** | Yes (Go) | No (Node.js) | Yes (Rust) | Yes (Go) | Yes (Go) | No (PHP) |
| **Per-project config** | Yes | No | Yes | No | No | No |

¹ Portless injects env vars into child processes but doesn't write to disk — values don't survive restarts or work with Docker Compose.
² dot-test writes `PORT` only — no derived values or multi-service wiring.
³ Outport writes `.env` and gets out of the way. Your existing dev tools (Foreman, Docker Compose, bin/dev) read it. No wrapper command needed.

### Why this matters for agentic development

If you're a single developer running one Rails app, most of these tools work fine. The differences show up when things get real:

- **Multiple projects at once** — three Rails apps all defaulting to port 3000, each with their own Postgres and Redis. You need them all running simultaneously, completely segregated.
- **Worktrees for parallel AI agents** — you tell three agents to work on three features, each in its own worktree. Every worktree needs a complete, non-conflicting set of ports for all services — web, database, cache, everything.
- **Multi-service apps** — your Nuxt frontend needs your Rails backend's URL. Your backend needs the frontend's URL for CORS. Outport's [derived values](#derived-values) wire this up declaratively — one config file, and every `.env` gets finished URLs.
- **Declare once, apply anywhere** — check `.outport.yml` into your repo. Every developer, every machine, every worktree gets deterministic ports with `outport apply`. No manual port bookkeeping, no "which port was that again?"

Outport handles the simple case (one app, one port) and scales to the complex case (monorepo, multiple services, parallel worktrees, agentic workflows) without changing your existing tools.

## Roadmap

- **v1 (current):** Port allocation + apply/unapply + `.env` writing
- **v2:** DNS server + reverse proxy for `.test` domains (`myapp.test` instead of `localhost:39519`)
- **v3:** Local SSL with real certificates for `.test` domains

## License

MIT

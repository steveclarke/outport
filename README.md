<p align="center">
  <a href="https://outport.app">
    <img src="brand/svg/logo-horizontal-color.svg" width="280" alt="Outport">
  </a>
</p>

# Outport

**Port orchestration for multi-project development.**

Outport allocates deterministic, non-conflicting ports for all your projects, assigns `.test` hostnames, and writes everything to `.env`. No more port conflicts. No more memorizing port numbers. No more cookie collisions across parallel instances.

## The Problem

You're running Rails on 3000, Nuxt on 5173, Postgres on 5432. You start a second project — port conflict. You spin up another instance for an AI agent — more conflicts. Your Nuxt frontend needs your Rails API URL. Your Rails backend needs the frontend URL for CORS. You're juggling port numbers across `.env` files, and if you get one wrong, nothing works.

Outport fixes this. Declare your services once in `outport.yml`, check it into your repo, and run `outport up`. Every developer, every machine, every instance gets deterministic ports — no coordination required.

## Install

### Homebrew

```bash
brew install steveclarke/tap/outport
```

### From Source

```bash
go install github.com/outport-app/outport@latest
```

### Build Locally

```bash
git clone https://github.com/steveclarke/outport.git
cd outport
go build -o outport .
```

## Quick Start

```bash
outport setup         # One-time setup (optional .test domains)
outport init          # Create outport.yml
outport up            # Allocate ports, write .env
```

After `outport up`, your `.env` has deterministic ports and your services are accessible at friendly hostnames:

```
myapp [main]

    web       PORT        → 24920  http://myapp.test
    postgres  DB_PORT     → 21536
    redis     REDIS_PORT  → 29454
```

## How It Works

Drop an `outport.yml` in your project:

```yaml
name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp.test
  postgres:
    env_var: DB_PORT
  redis:
    env_var: REDIS_PORT
```

Run `outport up`. Outport allocates a deterministic hash-based port (range 10000–39999) for each service and writes the result to `.env`. Services with `hostname` get a `.test` URL routed through a local reverse proxy.

### .test Domains

Run `outport system start` once to enable friendly hostnames. This installs a local DNS server, reverse proxy, and local CA — your services become accessible at `https://myapp.test` instead of `http://localhost:24920`.

The proxy runs via macOS launchd, starts at login, and updates routes automatically when you `outport up`. No port numbers in your browser, ever.

Open `https://outport.test` for a live dashboard showing all your projects, services, and health status.

### Multiple Instances

Every clone, worktree, or checkout of a project is an **instance**. The first is "main" — subsequent instances get auto-generated codes:

```
# Main checkout
$ outport up
myapp [main]
    web    PORT    → 24920  http://myapp.test

# Second clone / worktree — different ports, different hostname
$ outport up
  Registered as myapp-bkrm. Use 'outport rename bkrm <name>' to rename.
myapp [bkrm]
    web    PORT    → 28104  http://myapp-bkrm.test
```

Each instance gets its own ports and `.test` hostname. Cookie isolation is automatic — no incognito windows, no browser profile hacks.

### Multi-Service Hostnames

Projects with multiple HTTP services use subdomains:

```yaml
name: unio
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    hostname: unio.test
  frontend:
    env_var: FRONTEND_PORT
    protocol: http
    hostname: app.unio.test
  portal:
    env_var: PORTAL_PORT
    protocol: http
    hostname: portal.unio.test
  postgres:
    env_var: DB_PORT
```

Each HTTP service gets its own `.test` URL. Non-HTTP services (Postgres, Redis) get port allocations only.

## Computed Values

Applications don't just need port numbers — they need URLs. Computed values compute environment variables from your service map:

```yaml
computed:
  CORS_ORIGINS:
    value: "${frontend.url},${portal.url}"     # browser-facing: http://app.unio.test,...
    env_file: backend/.env
  API_URL:
    value: "${rails.url:direct}/api/v1"        # server-to-server: http://localhost:24920/api/v1
    env_file: frontend/.env
```

### Template Fields

| Template | Resolves to | Use case |
|----------|------------|----------|
| `${rails.port}` | `24920` | Raw port number |
| `${rails.hostname}` | `unio.test` | `.test` hostname |
| `${rails.url}` | `http://unio.test` | Browser-facing URLs (CORS, asset hosts) |
| `${rails.url:direct}` | `http://localhost:24920` | Server-to-server (API calls, WebSocket) |

Use `${service.url}` for URLs the browser sees. Use `${service.url:direct}` for server-to-server communication that bypasses the proxy.

### Per-File Overrides

When the same env var needs different values per file (common in monorepos):

```yaml
computed:
  API_BASE_URL:
    env_file:
      - file: frontend/apps/main/.env
        value: "${rails.url:direct}/api/v1"
      - file: frontend/apps/portal/.env
        value: "${rails.url:direct}/portal/api/v1"
```

## Multiple .env Files

Use per-service `env_file` to write ports to different locations — useful for monorepos:

```yaml
services:
  rails:
    env_var: RAILS_PORT
    env_file: backend/.env
  frontend:
    env_var: FRONTEND_PORT
    env_file: frontend/.env
```

Services without `env_file` write to `.env` in the project root. `env_file` can be a string or array to write to multiple files.

## Integration

Outport writes to `.env` because everything already reads it:

- **Docker Compose** reads `.env` automatically — use `${DB_PORT:-5432}` in `compose.yml`
- **Foreman / Overmind** reads `.env` — use `web: bin/rails server -p $PORT` in `Procfile.dev`
- **Rails** (dotenv-rails), **Nuxt**, **Phoenix**, **Django** — all have dotenv support
- **Any framework** that reads environment variables works with zero configuration

Outport preserves your existing `.env` variables. It only manages variables declared in `outport.yml` — everything else is untouched.

## Commands

### Project Commands

```
outport setup                  Interactive first-run system setup
outport init                   Create outport.yml for this project
outport up                     Allocate ports, assign hostnames, write .env
outport up --force             Clear and re-allocate all ports
outport down                   Remove ports, clean .env files
outport ports                  Show ports for the current project
outport ports --computed       Show ports and computed values
outport open                   Open HTTP services in the browser
outport qr                     Show QR codes for mobile device access
outport qr --tunnel            Show QR codes with tunnel URLs
outport share                  Tunnel HTTP services to public URLs
outport share web              Tunnel a specific service
outport rename <old> <new>     Rename an instance
outport promote                Promote the current instance to main
outport doctor                 Check the health of the outport system
```

### System Commands

```
outport system start           Install DNS, CA, and start the daemon
outport system stop            Stop the daemon
outport system restart         Re-write plist and restart the daemon
outport system status          Show all registered projects
outport system status --check  Show with health checks (up/down)
outport system gc              Remove stale registry entries
outport system uninstall       Remove DNS resolver, daemon, and CA
```

All commands support `--json` for machine-readable output. Use `--yes`/`-y` to auto-approve writing env files outside the project directory.

## AI Agent Skill

Install the Outport skill so your AI coding agent knows how to configure ports:

```bash
npx skills add steveclarke/outport/skills
```

The agent can run `outport up` in any instance, read `outport ports --json` for structured output, and configure `outport.yml` for new services.

## FAQ

### "My frontend and backend need to know each other's URLs, not just ports"

Use [computed values](#computed-values). `${service.url}` gives browser-facing URLs, `${service.url:direct}` gives server-to-server URLs. Outport resolves everything and writes finished values to `.env`.

### "I'm running two instances and my sessions are colliding"

Run `outport system start` to enable `.test` domains. Each instance gets its own hostname (`myapp.test` vs `myapp-bkrm.test`), so cookies are isolated automatically.

### "How do I add Outport to my project's setup script?"

Make it optional so developers without Outport aren't blocked:

```bash
if command -v outport > /dev/null 2>&1; then
  outport up
else
  echo "Outport not found — install: brew install steveclarke/tap/outport"
fi
```

## Development

Requires [Go 1.26+](https://go.dev/dl/) and [just](https://github.com/casey/just).

```bash
just build        # Build the binary
just test         # Run all tests
just lint         # Run linter
just run up       # Build and run with args
just clean        # Clean build artifacts
```

## Roadmap

- ~~**v1:** Port allocation + `.env` writing~~
- ~~**v2:** DNS server + reverse proxy for `.test` domains, instance model~~
- ~~**v3:** Local HTTPS with automatic certificates for `.test` domains~~
- ~~**v4:** QR codes for mobile device access~~
- ~~**v5:** Public URL sharing via Cloudflare Tunnel with multi-service orchestration~~
- **Next:** Linux support, team configuration sharing

## License

MIT

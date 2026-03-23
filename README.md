<p align="center">
  <a href="https://outport.dev">
    <img src="brand/svg/logo-horizontal-color.svg" width="280" alt="Outport">
  </a>
</p>

# Outport

**Port orchestration for multi-project development.**

Outport allocates deterministic, non-conflicting ports for all your projects, assigns `.test` hostnames with HTTPS, and writes everything to `.env`. No more port conflicts. No more memorizing port numbers. No more cookie collisions across parallel instances.

> **[Full documentation at outport.dev](https://outport.dev)**

## The Problem

You're running Rails on 3000, Nuxt on 5173, Postgres on 5432. You start a second project — port conflict. You spin up another instance for an AI agent — more conflicts. Your Nuxt frontend needs your Rails API URL. Your Rails backend needs the frontend URL for CORS. You're juggling port numbers across `.env` files, and if you get one wrong, nothing works.

Outport fixes this. Declare your services once in `outport.yml`, check it into your repo, and run `outport up`. Every developer, every machine, every instance gets deterministic ports — no coordination required.

## Install

```bash
brew install steveclarke/tap/outport
```

> [!TIP]
> You can also install [from source](https://outport.dev/guide/installation) with `go install`.

## Quick Start

```bash
outport setup         # One-time setup (optional .test domains + HTTPS)
outport init          # Create outport.yml
outport up            # Allocate ports, write .env
```

After `outport up`, your `.env` has deterministic ports and your services are accessible at friendly hostnames:

```
myapp [main]

    web       PORT        → 24920  https://myapp.test
    postgres  DB_PORT     → 21536
    redis     REDIS_PORT  → 29454
```

That's it. Outport writes finished environment variables to `.env` — every framework that reads `.env` works with zero configuration.

> [!NOTE]
> See the [Getting Started guide](https://outport.dev/guide/getting-started) for a full walkthrough.

## Features

### .test Domains with HTTPS

Run `outport system start` once to enable `.test` hostnames. This installs a local DNS server, reverse proxy, and CA — your services become accessible at `https://myapp.test` instead of `http://localhost:24920`. The proxy starts at login and updates routes automatically.

### Multiple Instances

Every clone, worktree, or checkout is an **instance**. The first is "main" — additional instances get auto-generated codes with their own ports and hostnames (`myapp-bkrm.test`). Cookie isolation is automatic.

### Computed Values

Services don't just need port numbers — they need URLs. Outport computes environment variables from your service map and writes finished values to `.env`. CORS origins, API URLs, WebSocket endpoints — declare them once in `outport.yml`.

See the [Configuration reference](https://outport.dev/reference/configuration) for template syntax and examples.

### Dashboard

Open `https://outport.test` for a live dashboard showing all your projects, services, ports, and health status. Updates in real-time as you run `outport up` across projects.

See the [Dashboard guide](https://outport.dev/guide/dashboard).

### Sharing and Mobile Access

`outport share` tunnels your HTTP services to public URLs via Cloudflare. `outport qr` shows QR codes for testing on mobile devices over your local network.

See [Tips & Troubleshooting](https://outport.dev/guide/tips) for details on tunneling and sharing.

### VS Code Extension

The [Outport for VS Code](https://outport.dev/guide/vscode) extension shows ports, URLs, and service health in the editor sidebar with clickable links.

### AI Agent Support

Install the Outport skill so your AI coding agent knows how to configure ports:

```bash
npx skills add steveclarke/outport/skills
```

See [Work with AI](https://outport.dev/guide/work-with-ai) for example prompts and what's included.

## FAQ

### "My frontend and backend need to know each other's URLs"

Use [computed values](https://outport.dev/reference/configuration#computed-values). `${service.url}` gives browser-facing URLs, `${service.url:direct}` gives server-to-server URLs. Outport resolves everything and writes finished values to `.env`.

### "My sessions are colliding across instances"

Enable `.test` domains with `outport system start`. Each instance gets its own hostname (`myapp.test` vs `myapp-bkrm.test`), so cookies are isolated automatically.

### "How do I add Outport to my project's setup script?"

Make it optional so developers without Outport aren't blocked:

```bash
if command -v outport > /dev/null 2>&1; then
  outport up
else
  echo "Outport not found — install: brew install steveclarke/tap/outport"
fi
```

## All Commands

All commands support `--json` for machine-readable output.

```
outport setup              One-time system setup
outport init               Create outport.yml
outport up [--force]       Allocate ports, write .env
outport down               Remove ports, clean .env
outport ports [--computed] Show allocated ports
outport open               Open services in the browser
outport qr [--tunnel]      QR codes for mobile access
outport share [service]    Tunnel services to public URLs
outport rename <old> <new> Rename an instance
outport promote            Promote instance to main
outport doctor             Diagnose issues
outport system start       Install DNS, HTTPS, start daemon
outport system stop|restart|status|gc|uninstall
```

See the [Commands reference](https://outport.dev/reference/commands) for full details.

## Development

Requires [Go 1.26+](https://go.dev/dl/) and [just](https://github.com/casey/just).

```bash
just build        # Build the binary
just test         # Run all tests
just lint         # Run linter
just run up       # Build and run with args
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

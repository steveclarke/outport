<p align="center">
  <a href="https://outport.app">
    <img src="brand/svg/logo-horizontal-color.svg" width="280" alt="Outport">
  </a>
</p>

# Outport

**Ports, hostnames, SSL, and sharing for local development. One command.**

Outport gives every project deterministic ports, a friendly hostname like `myapp.test`, real SSL certificates, and the ability to share your running app with anyone — even when your frontend talks to an API backend. No more port conflicts. No more typing `localhost:28104`. No more "let me share my screen."

```bash
brew install steveclarke/tap/outport
```

## The Problem

It's 2026. You're fixing a bug in one worktree while your AI agent builds a feature in another. Both need Rails, Postgres, and Redis — running simultaneously, with their own isolated stacks. Not "shut one down and start the other." Simultaneously.

Maybe one project needs Postgres 16 and the other needs 18. Maybe you're running a Vue frontend against a Rails API in one worktree, and a completely different project in another. Every service needs its own port, its own database, its own everything — and they all need to be running at the same time on your laptop.

Port 3000 is taken. Port 5432 is taken. You start changing port numbers, updating configs, trying to remember which project is on which port. Multiply that by worktrees and it becomes unmanageable.

But ports are just the start. You're building a mobile app that needs the phone camera — but the camera API requires HTTPS. You want to show your colleague what you've built — but your frontend talks to an API backend, so tunneling one service isn't enough. You spin up a worktree and now you're typing `localhost:28104` into your browser because who can remember that.

Outport is designed to solve all of this — from port allocation to hostnames, SSL, and sharing — so you can focus on the code instead of the wiring.

## Quick Start

```bash
cd ~/src/myapp
outport init              # Create outport.yml
outport up             # Allocate ports, write .env
```

That's it. Your services have deterministic ports. Once DNS and SSL are enabled, your app is at `https://myapp.test`.

## How It Works

Drop a `outport.yml` in your project that declares your services:

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

When you run `outport up`, Outport:

1. **Allocates ports** — assigns a deterministic hash-based port for each service so the same project always gets the same ports.
2. **Writes `.env`** — every framework already reads `.env` (Docker Compose, Foreman, Rails, Nuxt), so your services pick up their ports automatically.
3. **Registers everything** — a central registry at `~/.config/outport/` tracks all projects and worktrees so ports never collide across your entire machine.

Each worktree gets its own allocation with its own deterministic ports. Run `outport up` again — same result every time.

## What You Get

### Deterministic Ports

Every project and worktree gets stable, non-conflicting ports written to `.env`:

```bash
$ outport up
myapp
    web         PORT                 → 24920  http://localhost:24920
    postgres    DATABASE_PORT        → 21536
    redis       REDIS_PORT           → 29454

Ports written to .env
```

Ports are deterministic — same project, same worktree, same ports every time. Run `outport up` again — same result.

### Friendly Hostnames

Access your app at `myapp.test` instead of `localhost:24920`. Worktrees get their own hostnames automatically:

```
myapp.test                    → main checkout
myapp-feature-login.test      → feature-login worktree
mailpit.myapp.test            → Mailpit web UI
```

No more memorizing ports. Bookmark the hostname and it always works.

### Real SSL Certificates

`https://myapp.test` with browser-trusted certificates from Let's Encrypt. No mkcert, no self-signed certs, no "trust this certificate" prompts. Just HTTPS that works.

This means browser APIs that require secure contexts — camera, microphone, geolocation, Service Workers, Web Push — all work in development exactly like they do in production.

### Test on Your Phone

```bash
$ outport open --qr

  myapp — web

  █████████████████████
  █ ▄▄▄▄▄ █ ▄ █ ▄▄▄▄▄█
  █ █   █ █▄  █ █   █ █
  █ █▄▄▄█ █ █▄█ █▄▄▄█ █
  █████████████████████

  http://192.168.1.50:24920

  Scan with your phone (same WiFi)
```

Scan the QR code from your phone camera. You're in. No typing IP addresses.

### Share With Anyone

```bash
$ outport share

  myapp — web (port 24920)

  Public URL:  https://verb-noun-thing.trycloudflare.com

  Press Ctrl+C to stop
```

Your colleague in Botwood scans the QR code or clicks the link. They see your app running on your machine. Free via Cloudflare Tunnel, no signup required.

### Multi-Service Sharing

When your app has a frontend and a backend:

```yaml
name: myapp
services:
  frontend:
    env_var: VITE_PORT
    protocol: http
  api:
    env_var: PORT
    protocol: http
```

Running `outport share` tunnels **both** services and writes the tunnel URLs back into `.env`. Your Vue frontend picks up the API's tunnel URL. Your Rails backend picks up the frontend's URL for CORS. Everything just works.

This has been a persistent unsolved problem for the SPA + API era — you've never been able to just send someone a link to your app when your frontend talks to a backend. Outport is designed to fix that.

## Configuration

### Simple Project

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

### Multiple .env Files

Use per-service `env_file` to write to different locations:

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

Services default to `.env` in the project root if `env_file` is omitted. `env_file` can be a string or array to write the same var to multiple files.

## Worktree Support

Outport detects git worktrees automatically. Each worktree gets its own ports and hostname:

```bash
# Main checkout
$ outport up
myapp
    web         PORT                 → 24920  https://myapp.test
    postgres    DATABASE_PORT        → 21536

# Feature worktree — different ports, different hostname, zero conflicts
$ outport up
myapp [feature-login (worktree)]
    web         PORT                 → 18472  https://myapp-feature-login.test
    postgres    DATABASE_PORT        → 13567
```

Run as many worktrees as you want. No conflicts. No manual configuration.

## Integration

Outport writes to `.env` because everything already reads it:

- **Docker Compose** — `${DATABASE_PORT:-5432}` in `compose.yml`
- **Foreman / Overmind** — `web: bin/rails server -p $PORT` in `Procfile.dev`
- **Rails** (dotenv-rails), **Nuxt**, **Phoenix**, **Django** — all have dotenv support
- **Any framework** that reads environment variables

Outport only updates variables declared in your `outport.yml` — everything else in your `.env` is preserved.

## Commands

```
outport init              Create outport.yml for this project
outport up                Apply port configuration, write .env
outport up --force        Clear and re-allocate all ports
outport down              Remove from registry, free ports
outport ports             Show ports for the current project
outport open              Open HTTP services in the browser
outport open --qr         Show QR code for mobile access
outport share             Tunnel to a public URL via Cloudflare
outport share --qr        Show QR code for the tunnel URL
outport system start      Install DNS, CA, and start the daemon
outport system stop       Stop the daemon
outport system restart    Re-write plist and restart the daemon
outport system status     Show all registered projects
outport system status --check  Show with health checks (up/down)
outport system gc         Remove stale registry entries
outport system uninstall  Remove DNS resolver, daemon, and CA
```

All commands support `--json` for machine-readable output.

## How Ports Are Allocated

Outport uses FNV-32 hashing on `{project}/{instance}/{service}` to produce a deterministic port in the 10000-39999 range. Same inputs always produce the same port — so your project gets the same ports every time, on every machine.

Allocations are persisted in `~/.config/outport/registry.json`. Ports are stable: once allocated, `outport up` reuses the same ports. New services get fresh allocations without disturbing existing ones.

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

## Development

Requires [Go 1.24+](https://go.dev/dl/) and [just](https://github.com/casey/just).

```bash
just build        # Build the binary
just test         # Run all tests
just lint         # Run linter
just run register # Build and run with args
```

## Why "Outport"?

In Newfoundland, outports are the small, isolated coastal communities scattered along the shoreline — connected to each other and the wider world by water, not roads. For centuries, outport communities built remarkable things in remote places, staying connected despite the distance.

Outport is built by a remote team working from rural Newfoundland — about as far from Silicon Valley as you can get in North America. Sometimes from a cabin in the woods on Starlink. We built this tool because we needed it: when your team is spread across outports and your dev environment needs to reach across devices and distances, the wiring between services shouldn't be the hard part.

You don't need to be in a tech hub to build tools that connect the world. You just need a good internet connection and a problem worth solving.

## Contributing

Outport is open source under the MIT license. We'd love help building out the full vision — especially around DNS/proxy, SSL, and tunneling. Check the [issues](https://github.com/steveclarke/outport/issues) for what's planned.

## License

MIT

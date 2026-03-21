# DEVSTACK.md

`DEVSTACK.md` is a convention — a file committed to your project root that tells agents how to run your dev environment. `CLAUDE.md` tells agents how to work in a codebase. `DEVSTACK.md` tells them how to start it, stop it, and check if it's healthy.

## The problem

Agents can create files, run tests, make commits. But "spin up Postgres, wait for it to be healthy, then start the app server" is where they get stuck. Background process management is the gap in agentic coding.

As of early 2026, every major coding agent — Claude, Gemini, Codex — struggles with starting, monitoring, and stopping dev services. They don't own a terminal. They can't see a Foreman dashboard. They can't tell if Postgres is still booting or if the app server crashed on startup.

The missing piece isn't intelligence. It's a structured contract between the project and the agent that says: here's how to start everything, here's how to check if it worked, and here's how to shut it down.

## The convention

`DEVSTACK.md` is a file in your project root, committed to git. It has standard sections that any agent can parse:

```markdown
# DEVSTACK.md

Machine-readable dev environment reference for **my-project**.

## Prerequisites

| Tool            | Install                                        |
|-----------------|------------------------------------------------|
| Node.js 22      | `mise install` (reads `.node-version`)         |
| Docker Desktop  | https://www.docker.com/products/docker-desktop |
| outport         | `brew install steveclarke/tap/outport`         |
| process-compose | `brew install f1bonacc1/tap/process-compose`   |

## Setup

bin/setup

Installs dependencies, allocates ports, prepares databases.

## Start

bin/dev          # TUI mode (interactive terminal)
bin/dev -D       # Headless mode (for agents / background)

## Stop

bin/dev stop

## Health Check

bin/dev status

Returns JSON. Each entry has: name, status, is_ready, is_running.

## Logs

bin/dev logs <service>

## Restart

bin/dev restart <service>

## Worktrees

Each worktree gets its own ports and databases via outport.

## Notes

Project-specific gotchas go here.
```

The format is vendor-neutral. Claude, Gemini, Codex, or any future agent can read it. The sections are predictable — an agent knows exactly where to look for health check commands or log access.

## process-compose

The orchestration layer that makes this work is [process-compose](https://f1bonacc1.github.io/process-compose/) — a Go binary that manages your dev processes like docker-compose manages containers.

Why it's the right fit for agents:

- **Headless daemon mode** — `process-compose up -D` starts everything in the background. No terminal needed.
- **Kubernetes-style health checks** — exec and HTTP probes with readiness gates. An agent can wait for "Postgres is accepting connections" before starting the app server.
- **Dependency ordering** — declare that Rails depends on Postgres. process-compose won't start Rails until Postgres passes its health check.
- **JSON status output** — `process-compose process list --output json` returns structured data an agent can parse. No screen-scraping.
- **TUI dashboard** — when a human runs it interactively, they get a full terminal UI with logs, restarts, and process status.
- **Unix socket communication** — CLI commands (status, logs, restart) talk to the daemon over unix sockets, not HTTP. Multiple instances can run simultaneously.

Install:

```bash
brew install f1bonacc1/tap/process-compose
```

## The bin/dev pattern

`bin/dev` is already the standard command for starting a Rails dev server (and increasingly common in other frameworks). The idea is to keep the same command humans already know, but swap the backend from Foreman to process-compose.

Here's the complete wrapper script:

```bash
#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  -D)        shift; exec process-compose up -D --no-server "$@" ;;
  stop)      exec process-compose down ;;
  status)    exec process-compose process list --output json ;;
  logs)      exec process-compose process logs "${2:?specify a service}" ;;
  restart)   exec process-compose process restart "${2:?specify a service}" ;;
  *)         exec process-compose up --no-server "$@" ;;
esac
```

Each subcommand:

- **`bin/dev`** — launches the TUI for humans. Interactive dashboard with logs, process status, and restart controls.
- **`bin/dev -D`** — headless mode for agents. Starts all services in the background and returns immediately.
- **`bin/dev status`** — returns JSON status of every process. Agents use this to verify services are running and healthy.
- **`bin/dev logs <service>`** — tails logs for a single service. When something fails, an agent can read the logs to diagnose the issue.
- **`bin/dev restart <service>`** — restarts one service without touching the others. Useful after code changes that require a server restart.
- **`bin/dev stop`** — shuts everything down cleanly.

## How it works with outport

The full flow:

1. **`outport up`** — allocates deterministic ports and writes them to `.env`
2. **`bin/dev -D`** — process-compose reads `.env` automatically and starts all services on the correct ports
3. **`bin/dev status`** — agent verifies everything is healthy

Outport handles port allocation and `.env` generation. process-compose handles orchestration. Together they give agents a fully autonomous dev environment — no hardcoded ports, no port conflicts between worktrees, no guessing.

An agent working in a worktree runs the same three commands and gets an isolated environment with its own ports and databases. No configuration changes needed.

See [Getting Started](/guide/getting-started) for outport setup and [Work with AI](/guide/work-with-ai) for the outport AI skill.

## Gotchas we learned the hard way

### Login shell

**What goes wrong:** process-compose spawns a bare shell by default. Tools installed via Homebrew, mise, or Docker Desktop aren't on PATH. Processes fail with "command not found."

**Fix:** Configure a login shell in `process-compose.yml`:

```yaml
shell:
  shell_command: "bash"
  shell_argument: "-lc"
```

### Exec probes vs HTTP probes

**What goes wrong:** If your app is behind a reverse proxy (like outport's `.test` domains), HTTP probes hit port 80, get redirected to HTTPS, and fail with TLS certificate errors. The health check never passes.

**Fix:** Use exec probes with curl that hit the raw port directly:

```yaml
readiness_probe:
  exec:
    command: "curl -sf http://127.0.0.1:${PORT}/up"
```

### compose.yml naming conflict

**What goes wrong:** process-compose auto-discovers config files in this order: `compose.yml`, `compose.yaml`, `process-compose.yml`, `process-compose.yaml`. If you have a Docker `compose.yml` in the same directory, process-compose finds it first and chokes on Docker-specific keys like `volumes` and `image`.

**Fix:** Name your Docker file `docker-compose.yml` (the legacy name) so process-compose skips it.

### The --no-server flag

**What goes wrong:** process-compose starts an HTTP server on port 8080 by default. If you're running multiple worktrees simultaneously, they fight over that port and crash on startup.

**Fix:** Always pass `--no-server` when starting process-compose. CLI commands (status, logs, restart) use unix sockets, not HTTP, so nothing breaks. The `bin/dev` wrapper shown above already includes this.

### Theme config path on macOS

**What goes wrong:** You create `~/.config/process-compose/settings.yaml` and your theme doesn't apply.

**Fix:** process-compose uses XDG paths, which resolve to `~/Library/Application Support/` on macOS. The correct path is:

```
~/Library/Application Support/process-compose/settings.yaml
```

## Adopting `DEVSTACK.md`

`DEVSTACK.md` is a vendor-neutral convention, like `CLAUDE.md` and `AGENTS.md`. It works with any agent framework, any language, any stack.

If your project has a `bin/dev` or `docker-compose.yml` or `Procfile` — you already have the knowledge needed for a `DEVSTACK.md`. Write it down in a format agents can follow.

The pattern described on this page is a working implementation — process-compose health checks, Docker services, worktree support, and all the gotchas documented above. Use it as a starting point.

Add a `DEVSTACK.md` to your project. Commit it. Your agents will thank you by actually being able to run your code.

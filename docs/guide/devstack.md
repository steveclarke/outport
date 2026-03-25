---
description: A convention for telling AI agents how to start, stop, and health-check your dev environment using process-compose and Outport.
---

# Running Your Dev Stack

Outport allocates ports and writes `.env` files — it doesn't start or stop your services. But if you're using Outport, you'll also need a way to run your dev stack, and increasingly you'll want your AI coding agents to be able to start, stop, and health-check services on your behalf.

This page describes a complementary convention — `DEVSTACK.md` — that solves that problem. It's not part of Outport, but it pairs well with it. A file committed to your project root that tells agents how to run your dev environment. `CLAUDE.md` tells agents how to work in a codebase. `DEVSTACK.md` tells them how to start it, stop it, and check if it's healthy.

## The problem

Agents can create files, run tests, make commits. But "spin up Postgres, wait for it to be healthy, then start the app server" is where they get stuck. Background process management is the gap in agentic coding.

As of early 2026, every major coding agent — Claude, Gemini, Codex — can figure out how to start a service. But reliably coordinating multiple services — waiting for Postgres to accept connections before starting Rails, detecting a crashed process vs one that's still booting — requires a lot of hand-holding without a structured interface.

The missing piece isn't intelligence. It's a structured contract between the project and the agent that says: here's how to start everything, here's how to check if it worked, and here's how to shut it down. The approach below is the best solution we've found.

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

Each worktree gets its own ports, databases, and process-compose socket via outport.
Add `.pc_env` to `.gitignore`.

## Notes

Project-specific gotchas go here.
```

The format is vendor-neutral. Claude, Gemini, Codex, or any future agent can read it. The sections are predictable — an agent knows exactly where to look for health check commands or log access.

## process-compose (recommended)

The `DEVSTACK.md` convention is tool-agnostic, but we recommend [process-compose](https://f1bonacc1.github.io/process-compose/) as the orchestration layer. It's a Go binary that manages your dev processes like docker-compose manages containers.

Why it's the right fit for agents:

- **Headless daemon mode** — `process-compose up -D` starts everything in the background. No terminal needed.
- **Kubernetes-style health checks** — exec and HTTP probes with readiness gates. An agent can wait for "Postgres is accepting connections" before starting the app server.
- **Dependency ordering** — declare that Rails depends on Postgres. process-compose won't start Rails until Postgres passes its health check.
- **JSON status output** — `process-compose process list --output json` returns structured data an agent can parse. No screen-scraping.
- **TUI dashboard** — when a human runs it interactively, they get a full terminal UI with logs, restarts, and process status.
- **Unix socket communication** — CLI commands (status, logs, restart) talk to the daemon over unix sockets, not HTTP. Combined with `.pc_env`, multiple instances can run simultaneously with automatic socket isolation.

Install:

```bash
brew install f1bonacc1/tap/process-compose
```

## The bin/dev pattern

`bin/dev` is a common convention for starting a dev environment. This wrapper script gives it a consistent interface that both humans and agents can use:

Here's the complete wrapper script:

```bash
#!/usr/bin/env bash
set -euo pipefail

# process-compose reads .pc_env at startup and picks up PC_SOCKET_PATH,
# which outport writes with a per-instance value. This auto-enables UDS
# mode with a unique socket per worktree — no manual flags needed.

case "${1:-}" in
  -D)        shift; process-compose up -D "$@" ;;
  stop)      process-compose down ;;
  status)
    fmt="--output wide"; [[ "${2:-}" == "--json" ]] && fmt="--output json"
    process-compose process list $fmt 2>/dev/null \
      || { echo "Dev environment is not running. Start it with: bin/dev"; exit 1; }
    ;;
  logs)      process-compose process logs "${2:?specify a service}" ;;
  restart)   process-compose process restart "${2:?specify a service}" ;;
  *)         process-compose up "$@" ;;
esac
```

Each subcommand:

- **`bin/dev`** — launches the TUI for humans. Interactive dashboard with logs, process status, and restart controls.
- **`bin/dev -D`** — headless mode for agents. Starts all services in the background and returns immediately.
- **`bin/dev status`** — shows wide-format status of every process. Pass `--json` for machine-readable output. Agents use this to verify services are running and healthy.
- **`bin/dev logs <service>`** — tails logs for a single service. When something fails, an agent can read the logs to diagnose the issue.
- **`bin/dev restart <service>`** — restarts one service without touching the others. Useful after code changes that require a server restart.
- **`bin/dev stop`** — shuts everything down cleanly.

## Example process-compose.yml

A Rails app with Postgres. Outport writes the ports to `.env`, and process-compose reads them automatically:

```yaml
shell:
  shell_command: "bash"
  shell_argument: "-lc"

processes:
  postgres:
    command: >
      docker run --rm
      -p ${DB_PORT}:5432
      -e POSTGRES_PASSWORD=postgres
      -v myapp-pgdata:/var/lib/postgresql/data
      postgres:17
    readiness_probe:
      exec:
        command: "pg_isready -h 127.0.0.1 -p ${DB_PORT}"
      initial_delay_seconds: 2
      period_seconds: 2

  web:
    command: bin/rails server -p ${PORT}
    depends_on:
      postgres:
        condition: process_healthy
    readiness_probe:
      exec:
        command: "curl -sf http://127.0.0.1:${PORT}/up"
      period_seconds: 3
```

Key points:

- **Login shell** — `shell_argument: "-lc"` ensures Homebrew, mise, and Docker Desktop are on PATH
- **Dependency ordering** — Rails won't start until Postgres passes its health check
- **Readiness probes** — exec probes check the raw port directly, not through the `.test` proxy
- **No hardcoded ports** — everything reads from `.env`, so worktrees get isolated environments automatically

## How it works with Outport

The full flow:

1. **`outport up`** — allocates deterministic ports, writes them to `.env`, and writes `PC_SOCKET_PATH` to `.pc_env` for worktree-safe socket isolation
2. **`bin/dev -D`** — process-compose reads `.pc_env` (socket path) and `.env` (ports) automatically, starting all services on the correct ports with an isolated socket
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

### Worktree isolation with `.pc_env`

**What goes wrong:** When running multiple worktrees simultaneously, each needs its own process-compose socket. Without isolation, `bin/dev status` in one worktree talks to the wrong instance, and `bin/dev stop` shuts down the wrong stack.

**Fix:** process-compose loads `.pc_env` from the current directory at startup — before CLI flags, before `.env`, before anything else. Setting `PC_SOCKET_PATH` there auto-enables UDS mode with a unique socket per worktree.

Outport can write this file for you as a [computed value](/guide/getting-started#create-your-config):

```yaml
# outport.yml
computed:
  PC_SOCKET_PATH:
    value: "/tmp/process-compose-${project_name}${instance:+-${instance}}.sock"
    env_file: .pc_env
```

After `outport up`, each instance gets its own socket (e.g., `/tmp/process-compose-myapp.sock` for main, `/tmp/process-compose-myapp-wiki.sock` for a worktree). All `process-compose` and `bin/dev` commands find the right socket automatically — no flags needed. Add `.pc_env` to your `.gitignore`.

### Socket path out of sync

**What goes wrong:** You renamed an instance or ran `outport down` and `outport up` in a different directory. The `.pc_env` file still has the old socket path, so `process-compose` commands can't find the running daemon.

**Fix:** Run `outport up` to regenerate `.pc_env` with the current socket path. If you need to stop a running stack with a stale socket, pass the path directly: `process-compose down -u /tmp/process-compose-<old-name>.sock`.

## Adopting `DEVSTACK.md`

`DEVSTACK.md` is not an official standard — no agent picks it up automatically today. You'll need to point your `CLAUDE.md` or `AGENTS.md` to it (e.g., "See DEVSTACK.md for how to start and stop services"). Our hope is that it could become a recognized convention, like `CLAUDE.md` and `AGENTS.md` are now.

If your project has a `bin/dev` or `docker-compose.yml` or `Procfile` — you already have the knowledge needed for a `DEVSTACK.md`. Write it down in a format agents can follow.

The pattern described on this page is a working implementation — process-compose health checks, Docker services, worktree support, and all the gotchas documented above. Use it as a starting point.

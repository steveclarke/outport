---
description: Use the Outport skill with Claude Code, Gemini CLI, and other AI coding agents for port setup, config authoring, and troubleshooting.
---

# Work with AI

Outport provides an official skill for AI coding agents, so they know how to configure ports, set up `outport.yml`, and troubleshoot your dev environment.

## Install the Skill

```bash
npx skills add steveclarke/outport/skills
```

This works with Claude Code, Gemini CLI, and other agents that support the skills protocol.

## Example Prompts

Once installed, your AI agent understands Outport's config format, commands, and conventions. Try prompts like:

**Set up a new project:**
> "Set up outport for this project. We have a Rails API, Postgres, and Redis."

**Add a service to an existing config:**
> "Add a Nuxt frontend to our outport.yml. It needs to know the Rails API URL for server-side fetches and the browser-facing URL for CORS."

**Diagnose issues:**
> "My .test domains stopped working. Can you figure out what's wrong?"

**Configure a monorepo:**
> "We have a Rails backend and two Nuxt frontends in a monorepo. Each app has its own .env file. Set up outport so they can discover each other's URLs."

**Share with a teammate:**
> "I need to share my local dev server with a teammate. Set up a public URL."

## What's Included

The skill covers:

- `outport.yml` configuration — services, hostnames, preferred ports, env files
- Computed values — template syntax, `url` vs `url:direct`, per-file overrides, `${instance}` variable
- All CLI commands — `up`, `down`, `ports`, `open`, `share`, `doctor`, `system start/stop/restart`
- Multi-instance workflows — worktrees, renaming, promoting
- Troubleshooting — DNS, daemon, certificates, port conflicts

## Machine-Readable Output

Every outport command supports `--json` for structured output that agents can parse:

```bash
outport ports --json      # current port allocations
outport doctor --json     # system health check results
outport up --json         # allocation results after setup
```

Agents use this to read your project's port map, verify system health, and confirm changes without parsing terminal output.

## Full Stack Orchestration

For running your entire dev environment — database, web servers, background workers — see the [`DEVSTACK.md` guide](/guide/devstack). It covers process-compose, health checks, and headless operation for AI agents.

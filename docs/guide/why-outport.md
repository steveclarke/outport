# Why Outport?

I *wanted* to build this tool for years. I *had* to build it in the agentic era.

## The agency developer's reality

I work at a small company in rural Newfoundland. We're essentially an agency — web apps, client projects, internal tools. On any given day I'm switching between three or four projects. Different Rails versions, different stacks, different databases. Project A is on port 3000 with Postgres on 5432. A client calls needing a quick fix on another app. Shut everything down, spin up Project B, maybe go 3001 or 3002 to avoid conflicts. Come back a month later — "what port was this on again?"

Meanwhile I want things running simultaneously. I'm waiting for tests on one project while working on another. But that means picking ports that don't collide, updating every config that references them, and hoping I remember what I changed. All work stops when ports conflict. It's a small thing, but it compounds.

## The forgotten middle

Hobbyists don't have this problem. One project at a time, `rails s`, done. Big companies don't have it either — they have dedicated devtools engineers who build internal tooling for exactly this kind of thing.

The people who juggle the most projects have the fewest tools. Every developer tool on the internet assumes you're a Silicon Valley startup with one product or a hobbyist with one side project. Nobody builds for the people in the middle — the small shops and agencies who'd benefit the most.

## Living with the pain

For years, this was manageable friction. Not a crisis. I changed ports manually, kept mental notes, shut things down and spun them back up. It was never worth building a tool over. The time to build it would have exceeded the time saved by years. "Just change the port, Steve." That's one of the downsides of working at a small shop. You live with things.

## The agentic era changed the math

Two shifts happened at once. First: AI coding agents can build the tool. What would have taken months of solo dev time is now feasible. Second, and more important: agents *need* the tool.

With four or five Claude Code sessions open, working on multiple projects and multiple instances of the same project simultaneously, port conflicts went from occasional annoyance to constant blocker. Agents can't read an error message and pick a different port. They can't keep a mental spreadsheet. They need deterministic, non-conflicting ports — declared once and guaranteed.

The ROI flipped from "not worth building" to "can't work without it."

Agents also need to know how to run your stack — that's what [DEVSTACK.md](/guide/devstack) solves.

## "Just change the port"

And sure, you can change the port. But it's never just the port.

Change the Rails port and now your Nuxt frontend is pointing at the wrong API URL. Your CORS config is rejecting requests from the new origin. Your [Bruno](https://www.usebruno.com/) collections are hitting the old port. Your Docker Compose stack is still trying to bind to 5432 even though another project has it. Your WebSocket URL is wrong. Your asset host is wrong.

A real app might have a Rails API, two frontend apps, Postgres, Redis, Mailpit, and Bruno for API testing. Each one has a port. Each one has config that references other services' ports. Change one and you're chasing a dozen config files across three `.env` files. Now multiply that by worktrees — each checkout needs its own completely isolated set of ports, URLs, CORS origins, and Docker containers.

That's not a port problem. That's an orchestration problem. And it's the kind of thing that's invisible if you've only ever worked on one app at a time.

## The name

An outport is a small, isolated community on the coast of Newfoundland. That's where this was built, and that's who it's for — developers working outside the big-company bubble where these problems get solved by dedicated teams.

---

## How Outport Compares

| Feature | Outport | Portless (Vercel) | puma-dev / Valet | Docker Compose | mkcert + Caddy |
|---|---|---|---|---|---|
| Deterministic ports | Yes | No (random) | No | Manual | No |
| Multi-service config | Yes | One app at a time | One app at a time | Yes | Manual per project |
| .env generation | Yes (fenced blocks) | No | No | No | No |
| Computed values | Yes (`${service.url}`) | No | No | No | No |
| .test hostnames | Yes | .localhost | Yes | No | Yes (manual config) |
| Auto HTTPS | Yes | Yes | Yes | No | Yes |
| Multi-instance / worktrees | Yes | Detects worktrees | No | No | No |
| Language-agnostic | Yes | JS-focused | Ruby / PHP | Yes | Yes |

## When to Use What

**Docker Compose** — Great if your team is already containerized. Outport complements Docker by managing host-side port mappings and `.env` files. See the [Docker Compose example](/guide/examples#docker-compose-multi-instance) for how they work together.

**Portless** — Solid for single-app workflows with named URLs. Outport is designed for multi-service projects with cross-service URL wiring and `.env` generation.

**puma-dev / Valet** — Good `.test` domain support for Ruby or PHP. Outport adds port orchestration, `.env` generation, computed values, and works with any language or framework.

**mkcert + Caddy** — Handles local HTTPS well with manual configuration per project. Outport bundles that functionality and adds port allocation, computed values, and multi-instance support — all from a single config file.

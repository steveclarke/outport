# Why Outport?

## The Problem

You're running Rails on 3000, a Nuxt frontend on 5173, Postgres on 5432. You start a second project — port conflict. You switch to a worktree — same ports. You change a port, and now your CORS config, API URLs, and Docker Compose port mappings are all wrong.

Most developers solve this with a mental spreadsheet and `kill -9`. It works until it doesn't.

Outport replaces that with a single config file. Declare your services once and every checkout gets deterministic ports, `.test` hostnames, and fully wired `.env` files.

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

**Docker Compose** — Great if your team is already containerized. Outport complements Docker by managing host-side port mappings and `.env` files. See the [Docker Compose example](/guide/examples#docker-compose-integration) for how they work together.

**Portless** — Solid for single-app workflows with named URLs. Outport is designed for multi-service projects with cross-service URL wiring and `.env` generation.

**puma-dev / Valet** — Good `.test` domain support for Ruby or PHP. Outport adds port orchestration, `.env` generation, computed values, and works with any language or framework.

**mkcert + Caddy** — Handles local HTTPS well with manual configuration per project. Outport bundles that functionality and adds port allocation, computed values, and multi-instance support — all from a single config file.

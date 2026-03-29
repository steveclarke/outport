---
description: What actually happens when you run outport up, how ports are allocated, and how .test domains reach your services.
---

# How It Works

You've run `outport up` and your services got ports. You've visited `myapp.test` and it worked. This page explains what happened behind the scenes.

## What happens when you run `outport up`

### 1. Find the config

Outport walks up from your current directory looking for `outport.yml`. This is what defines your project — which services you have, what env vars they need, and whether they get `.test` hostnames. Once found, that directory becomes the project root.

### 2. Identify the instance

Every checkout of a project is an **instance**. The first checkout is called "main." If you create a git worktree or clone the repo to a second directory, Outport detects that the project name is already registered and assigns a short 4-character code (like `bxcf`) to the new checkout.

Outport figures this out by looking up your directory path in the registry. If the path isn't registered but the project name already is, you're in a new instance.

Instances matter because each one gets its own ports, its own hostnames, and its own env files. You can run the main checkout and a worktree side by side without conflicts.

### 3. Allocate ports

If this project is already registered, Outport preserves the existing port allocations. Only new services (ones that weren't in the registry before) get allocated.

For new allocations, Outport hashes the combination of project name, instance, and service name to produce a port number. The same inputs always produce the same port — so `outport up` is safe to run repeatedly. Ports land in the 10000–39999 range, and if two services happen to hash to the same port, Outport bumps to the next available one.

### 4. Save to the registry

The **registry** is a JSON file at `~/.local/share/outport/registry.json`. It stores every project and instance along with their allocated ports, hostnames, and env var names. This is what makes allocations persistent — once a port is assigned, it stays assigned until you run `outport down` or `outport up --force`.

The registry is also what the daemon watches. When you run `outport up`, the daemon picks up the change and updates its route table so `.test` hostnames point to the right ports.

### 5. Write env files

Outport writes your allocated ports and computed values into your env files (`.env` by default, but configurable per service). It uses a **fenced block** — a clearly marked section that looks like this:

```
# Your own variables up here are never touched
MY_CUSTOM_VAR=hello

# --- begin outport.dev ---
PORT=12345
DB_PORT=12346
# --- end outport.dev ---
```

Everything between the markers is managed by Outport. Everything outside is yours. If you define a variable like `PORT=3000` above the block and Outport also manages `PORT`, it will move your definition into the managed block so there's a single source of truth.

### 6. Expand templates

If your config has computed values — like a `DATABASE_URL` that references `${db.port}`, or a `BASE_URL` that uses `${web.url}` — Outport resolves them now. Template variables are expanded using the ports and hostnames that were just allocated, and the results are written into the env files alongside the ports.

## What happens when you visit `myapp.test`

When your browser navigates to `myapp.test`, three things happen in quick succession:

1. **DNS** — Your OS is configured to send all `.test` domain lookups to Outport's daemon (running on port 15353). On macOS this is a resolver file; on Linux it's a systemd-resolved drop-in config. The daemon responds with `127.0.0.1` — every `.test` hostname points to your own machine.

2. **Routing** — Your browser connects to port 80 (HTTP) or 443 (HTTPS) on localhost. The daemon inspects the hostname from the request, looks it up in its route table, and proxies the request to the correct local port. If `myapp.test` maps to port 12345, your request lands on port 12345.

3. **HTTPS** — For secure connections, the daemon has a local Certificate Authority that issues certificates on the fly. Your browser trusts these because `outport setup` added the CA to your system trust store (and on Linux, to browser-specific certificate databases). The result is real HTTPS with a padlock icon, all running locally.

This is why `.test` hostnames work without any changes to `/etc/hosts` and why HTTPS just works without self-signed certificate warnings.

::: details What are DNS, daemons, and certificate authorities?
**DNS** (Domain Name System) translates names like `myapp.test` into IP addresses. Normally your ISP handles this, but Outport runs a tiny local DNS server just for `.test` domains — it tells your browser "that's at 127.0.0.1" (your own machine).

A **daemon** is a program that runs in the background. Outport's daemon handles DNS lookups and proxies web requests to the right port. Your OS manages it automatically — you don't need to start it manually.

A **Certificate Authority** (CA) is what makes HTTPS work. Browsers only show the padlock icon if the certificate was signed by a trusted authority. Outport creates a private CA on your machine and tells your OS to trust it, so your `.test` domains get real HTTPS locally. This CA only exists on your machine and only signs certificates for `.test` domains.
:::

## Key concepts

**Registry** — The JSON file (`~/.local/share/outport/registry.json`) that stores all port allocations, hostnames, and env var names for every project and instance. This is the source of truth that persists between runs and that the daemon watches for changes.

**Instance** — A single checkout of a project. The first checkout is "main" and gets clean hostnames like `myapp.test`. Additional checkouts (worktrees, clones) get a short code suffix like `myapp-bxcf.test`. Each instance has its own ports, hostnames, and env files.

**Fenced block** — The `# --- begin/end outport.dev ---` section in your env files. Outport only writes inside this block. Your own variables outside it are never modified.

**Daemon** — A background process managed by your operating system (launchd on macOS, systemd on Linux). It runs a DNS server and HTTP/HTTPS proxies that make `.test` domains work. You interact with it through `outport system` commands, but most of the time it runs silently. The daemon watches the registry and updates its routes automatically when you run `outport up` or `outport down`.

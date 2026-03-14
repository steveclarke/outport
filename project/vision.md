# Outport — Vision

## The Problem

Modern development means running multiple projects on one machine — Rails apps, Vue frontends, Docker services. Each needs its own ports. With agentic coding and git worktrees, you might run 3-5 instances of the *same* project at once. Port conflicts are constant.

But ports are just the beginning. The real friction is everything that comes after:

- You're building a mobile inventory app that uses the phone camera. You're developing on your desktop, but the camera API requires HTTPS. How do you test it on your phone?
- You want to show your colleague in Botwood what you've built. You tunnel your frontend — but it talks to an API backend. They can see your UI, but every API call fails. You'd need to tunnel the backend too, update the frontend's API URL, configure CORS... so you just share your screen on a call instead.
- You spin up a worktree for a feature branch. Now you need a whole new set of ports, new `.env` values, and you're typing `localhost:28104` into your browser because who can remember that.

These problems have existed for years. Every developer has hit them. Outport is designed to solve them end-to-end.

## The Vision

**Outport makes your dev environment work the way it should — across every project, every device, every worktree.**

Drop a config file in your project. Run `outport register`. You get:

- **Deterministic ports** that never conflict, across all your projects and worktrees
- **A friendly hostname** like `myapp.test` instead of `localhost:28104`
- **Real SSL certificates** so browser APIs (camera, geolocation, Service Workers) just work
- **A QR code** you scan from your phone to open the app instantly
- **A public URL** you send to your colleague so they can see what you've built
- **Multi-service orchestration** so your Vue frontend and Rails API automatically discover each other's URLs — even through tunnels

One tool. One config file. Every device. Zero thinking.

## The Stack

Each layer builds on the one before it:

### 1. Port Allocation (done)
Each project and worktree gets deterministic, non-conflicting ports. Outport writes them to `.env` — the universal integration point that Docker Compose, Foreman, Rails, Nuxt, and everything else already reads.

### 2. Friendly Hostnames
Local DNS server + reverse proxy. `myapp.test` instead of `localhost:28104`. Worktrees get their own hostnames automatically. No more memorizing ports — bookmark the hostname and it always works.

### 3. SSL Certificates
Real, browser-trusted certificates via Let's Encrypt DNS-PERSIST-01. Since Outport controls the DNS server, it can handle the ACME challenge automatically. `https://myapp.test` works with no manual certificate setup — which means camera APIs, secure cookies, and Service Workers all work in development.

### 4. QR Code
`outport open --qr` prints a QR code to your terminal. Scan it from your phone, you're in. No typing IP addresses and port numbers into a mobile browser. Works over the local network — your phone just needs to be on the same WiFi.

### 5. Share
`outport share` tunnels your service to a public URL via Cloudflare Tunnel. Free, no signup. Your colleague in Botwood scans the QR code or clicks the link and sees your app running on your machine. Works from anywhere — no VPN, no firewall configuration, no port forwarding.

### 6. Multi-Service Orchestration
The feature nobody else can build. When you `outport share` a project with a Vue frontend and a Rails API, Outport tunnels both services and writes the tunnel URLs back into `.env`. Your frontend picks up the API's tunnel URL from its environment. Your backend picks up the frontend's tunnel URL for CORS. Everything just works — because Outport owns the entire service map and every `.env` file.

This has been an unsolved problem for the entire SPA + API era. You've never been able to say "here's a link to my app" when your app is a frontend that talks to a backend — unless you deployed it. Outport fixes that.

## Why Outport Can Do This

The key insight is that Outport sits at the intersection of three things no other tool controls simultaneously:

1. **The service map** — it knows every service in every project
2. **The environment files** — it writes the `.env` that configures every service
3. **The network layer** — DNS, proxy, tunnels

Because it owns all three, it can do things that are impossible when these concerns are handled by separate tools. ngrok can tunnel a port but doesn't know your frontend needs your backend's URL. Your process manager reads `.env` but doesn't know about tunnels. Outport connects all of it.

## Design Principles

1. **Zero thinking** — `outport register` and forget about it
2. **Convention over configuration** — sensible defaults, config only when needed
3. **`.env` is the contract** — the universal integration point
4. **Framework-agnostic** — Rails, Nuxt, Phoenix, Django, anything with env vars
5. **Worktree-native** — first-class support for parallel development
6. **Single binary** — Go, no runtime dependencies, install via Homebrew
7. **Every device** — your desktop, your phone, your colleague's laptop

## Tech Stack

- **Go** — single binary, no runtime dependencies
- **CertMagic** — Caddy's certificate management library, embeddable in Go
- **Cloudflare Tunnel** — free tunneling infrastructure, no self-hosting
- **Homebrew** — primary distribution
- **Domain:** outport.app

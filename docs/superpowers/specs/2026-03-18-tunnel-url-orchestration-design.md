# Tunnel URL Orchestration Design

> **For agentic workers:** This spec describes enhancing `outport share` to rewrite `.env` files with tunnel URLs so multi-service architectures (SPA + API) automatically discover each other's public URLs during sharing.

**Goal:** When `outport share` tunnels HTTP services, computed values like CORS origins and API base URLs should automatically resolve to tunnel URLs — and revert when sharing stops.

**Depends on:** Issue #16 (`outport share` — basic tunneling, shipped in v0.15.0)

---

## Problem

When a project has a frontend and a backend, tunneling one service isn't enough. The frontend needs the backend's tunnel URL for API calls, and the backend needs the frontend's tunnel URL for CORS. Today this requires manually copying tunnel URLs into env vars and restarting services. Nobody does this.

Outport is uniquely positioned to solve this because it already owns the full service map AND the `.env` files. The computed values system (`${service.url}`) already wires services together — during sharing, those URLs should resolve to tunnel URLs instead of local `.test` URLs.

## Design

### Core Mechanic

`outport share` reuses the existing env-writing pipeline from `outport up`. The only change is what `${service.url}` resolves to for tunneled services.

**Normal state** (`outport up`):
- `${rails.url}` → `https://api.unio.test`
- `${rails.url:direct}` → `http://localhost:24920`

**During sharing** (`outport share`):
- `${rails.url}` → `https://abc-def-ghi-jkl.trycloudflare.com`
- `${rails.url:direct}` → `http://localhost:24920` (unchanged)

The `url:direct` field stays as localhost because local services still talk to each other on the same machine. Only browser-facing URLs (`${service.url}`) change to tunnel URLs.

After overriding the template vars, the pipeline runs unchanged:

1. `buildTemplateVars()` — with tunnel URL overrides for shared services
2. `ResolveComputed()` — expands all computed value templates
3. `mergeEnvFiles()` — rewrites fenced blocks in `.env` files

Every computed value that references `${service.url}` cascades automatically. CORS origins, API base URLs, asset hosts — all resolve to tunnel URLs without any config changes.

### Refactoring the Pipeline

The env-writing pipeline (`buildTemplateVars` → `ResolveComputed` → `mergeEnvFiles`) currently lives as unexported functions in `cmd/up.go`. To share it between `up` and `share`, extract it into a common function:

```
computeAndWriteEnv(cfg, reg, alloc, tunnelURLs map[string]string) error
```

- `tunnelURLs` is nil during `outport up` (normal behavior)
- `tunnelURLs` is populated during `outport share` (tunnel URL overrides)
- When a service appears in `tunnelURLs`, `buildTemplateVars` uses the tunnel URL for `${service.url}` instead of the `.test` URL

Both `up` and `share` call this function. No logic duplication.

### Cleanup on Exit

When the user hits Ctrl+C:

1. Stop all tunnels (existing behavior)
2. Re-run the env-writing pipeline **without** tunnel overrides — identical to running `outport up`
3. `.env` files are restored to local URLs

If cleanup fails (kill -9, power loss), `.env` files retain dead tunnel URLs. Running `outport up` manually restores local state. This is the standard recovery path.

### User Experience

**On start** (after tunnels are established and `.env` rewritten):

```
Sharing 3 services:

    rails           → https://abc-def-ghi-jkl.trycloudflare.com
    frontend_main   → https://mno-pqr-stu-vwx.trycloudflare.com
    frontend_portal → https://yza-bcd-efg-hij.trycloudflare.com

Updated .env files with tunnel URLs.
Restart your services to pick up the new URLs.

Press Ctrl+C to stop sharing.
```

**On stop** (Ctrl+C):

```
Stopping tunnels...
Restored .env files to local URLs.
Restart your services to revert to local development.
```

**`--json` mode**: Includes `tunnelUrls` map and shows which computed values were rewritten.

### Partial Sharing

When sharing a subset of services (`outport share web`), only those services get tunnel URL overrides. Non-tunneled services retain their local `.test` URLs. This means computed values referencing non-tunneled services still point to local URLs — which is correct, since those services aren't reachable from outside the network anyway.

### Error Handling

- **Partial tunnel failure**: The tunnel manager has all-or-nothing semantics. If one tunnel fails, none start and `.env` is never rewritten.
- **No computed values configured**: The pipeline still runs — service port env vars are written as normal. Tunnel URLs appear in the share output for manual use.
- **Config changed while sharing**: The exit cleanup re-reads config and recomputes everything fresh, picking up any changes.

## What Doesn't Change

- **No new config fields.** No `tunnel_url_var`, no `tunnel:` section. The existing `services` and `computed` config is sufficient.
- **No new template variables.** `${service.url}` is dynamic based on context — no `${service.tunnel_url}`.
- **No restart hooks.** The user restarts their own services. Messages tell them when to do so.
- **No persistent tunnel state.** Tunnel URLs exist only while `outport share` is running. Nothing is written to the registry.
- **The `up` command is unchanged.** It never knows about tunnels. The shared pipeline gains an optional tunnel URL override parameter.

## Concrete Example: Unio

Unio has Rails API (`api.unio.test`), Nuxt main app (`unio.test`), and Nuxt portal (`portal.unio.test`). Computed values include:

- `CORE_CORS_ORIGINS` = `${frontend_main.url},${frontend_portal.url}` → written to `backend/.env`
- `NUXT_API_BASE_URL` = `${rails.url:direct}/api/v1` → written to `frontend/apps/main/.env`

After `outport share`:

- `CORE_CORS_ORIGINS` = `https://ghi-jkl.trycloudflare.com,https://mno-pqr.trycloudflare.com` — Rails accepts CORS from tunnel origins
- `NUXT_API_BASE_URL` = `http://localhost:24920/api/v1` — unchanged, because it uses `url:direct` and Nuxt still calls Rails on localhost

A remote viewer opens the frontend tunnel URL. Their browser loads the Nuxt app (served via tunnel). Client-side API calls use the Rails tunnel URL (configured via browser-facing env vars). Server-side rendering uses localhost (via `url:direct`). Everything works.

# Wildcard Subdomain Routing via `subdomains: true`

**Date:** 2026-04-09
**Issue:** #103
**Status:** Design approved

## Summary

Add a `subdomains` boolean to service definitions that routes all subdomains of the primary hostname to the same port, without requiring explicit aliases for each one. This enables multi-tenant apps that use subdomains to identify tenants (e.g., `rp.realty120.test`, `tina-snow.realty120.test`) without maintaining a growing alias list in `outport.yml`.

## Motivation

Multi-tenant apps use subdomains for tenant routing. Currently each subdomain needs an explicit alias — every new tenant means editing `outport.yml` and rerunning `outport up`. The alias list mirrors the database, which defeats the purpose. A single boolean covers the common case: all subdomains of a hostname route to the same port.

## Config

```yaml
services:
  web:
    env_var: PORT
    hostname: realty120.test
    subdomains: true
```

A `Subdomains bool` field on the `Service` struct. When `true`, all subdomains of the primary hostname (`*.realty120.test`) route to the same port.

### Validation

- `subdomains: true` requires a primary `hostname` — error without one.
- No changes to hostname validation itself.
- Local override (`outport.local.yml`) can set `subdomains: true` on a service, following existing field-level merge rules.

### Scope

- Applies to the **primary hostname only**, not aliases. Aliases are explicit by nature — wildcarding `*.admin-panel.realty120.test` (three levels deep) has no real use case.
- Explicit aliases remain available for routing specific subdomains to a *different* port, and for making specific subdomains tunnelable via `outport share`.

## Registry

Add `Subdomains map[string]bool` to the `Allocation` struct. Omitted from JSON when empty (`omitempty`).

```json
{
  "ports": { "web": 10384 },
  "hostnames": { "web": "realty120.test" },
  "subdomains": { "web": true }
}
```

A new `ComputeSubdomains` function in the allocation package copies the boolean flag from config services into the allocation.

Instance suffixing works naturally: main instance wildcards `*.realty120.test`, instance `bkrm` wildcards `*.realty120-bkrm.test` — the wildcard is always derived from the computed hostname.

## Proxy Routing

### RouteTable changes

Add a `wildcards map[string]route` field to `RouteTable`, alongside the existing `routes` map. Keys are parent hostnames (e.g., `realty120.test`), values are `route` structs.

### Route building

`BuildRoutes` returns both maps (signature changes to return `(map[string]route, map[string]route)`):

```go
routes["realty120.test"] = route{Port: 10384}       // exact (always)
wildcards["realty120.test"] = route{Port: 10384}     // wildcard (when subdomains is true)
```

### Lookup

Exact match first, wildcard fallback on miss:

```go
func (rt *RouteTable) Lookup(hostname string) (route, bool) {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    if r, ok := rt.routes[hostname]; ok {
        return r, true
    }
    if idx := strings.Index(hostname, "."); idx > 0 {
        parent := hostname[idx+1:]
        if r, ok := rt.wildcards[parent]; ok {
            return r, true
        }
    }
    return route{}, false
}
```

Exact matches always win — `api.realty120.test` as an explicit hostname or alias beats the wildcard. Zero cost on exact-match hits. One extra map lookup on misses.

### Conflict example

```yaml
web:
  hostname: realty120.test
  subdomains: true           # *.realty120.test → port A

api:
  hostname: api.realty120.test   # exact match wins → port B
```

This is valid and requires no special validation. The proxy handles it naturally via lookup precedence.

## TLS Certificates

No changes needed. The cert manager already generates certs lazily via a `GetCertificate` SNI callback. New subdomains like `admin.realty120.test` get certs generated and cached on first request.

## CLI Output

`outport up` and `outport status`:

```
web  PORT → 10384  https://realty120.test (+ subdomains)
```

`--json` output: include `"subdomains": true` in the service data where applicable, following the existing envelope pattern.

## Dashboard

Add a "(+ subdomains)" label next to the hostname for services with the flag enabled. The `subdomains` field flows through the existing allocation data — no new API endpoints needed.

## Tunnels (`outport share`)

Wildcard subdomain routing is **local-only**. Tunnels require one `cloudflared` process per hostname, so dynamic wildcards cannot be tunneled.

Behavior:
- `outport share` tunnels the primary hostname and explicit aliases as usual, ignoring the wildcard.
- When a service has `subdomains: true`, print a note:
  ```
  Note: subdomain routing for realty120.test is local-only (tunnels use explicit hostnames)
  ```
- Users who need a specific subdomain reachable externally can add it as an explicit alias (which gets tunneled normally) or run their own `cloudflared` instance.

## Testing

Table-driven tests, `t.TempDir()` for filesystem isolation, no mocks:

- **Config validation:** `subdomains: true` without hostname → error; with hostname → valid; local override merge works.
- **Route building:** service with `subdomains: true` populates both `routes` and `wildcards` maps.
- **Lookup:** exact match wins over wildcard; wildcard matches arbitrary subdomains; non-matching hostnames miss; no wildcard without the flag.
- **Precedence:** service A with `subdomains: true` on `realty120.test`, service B with explicit `api.realty120.test` — exact match wins.
- **Instance suffixing:** non-main instance wildcards `*.realty120-bkrm.test`.
- **Allocation:** `ComputeSubdomains` copies flags from config to registry.

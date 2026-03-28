# Hostname Aliases — Design Spec

**Issue:** [#79 — Support multiple hostnames per service](https://github.com/steveclarke/outport/issues/79)
**Date:** 2026-03-26

## Problem

A single service (e.g., a Rails app) may need multiple `.test` hostnames routed to the same port. For example, a Rails app serving both a marketing site on `approvethis.test` and an app dashboard on `app.approvethis.test`. Both are the same process on the same port — Rails uses the `Host` header to determine which content to serve.

Currently, each service gets exactly one `hostname`. Registering a second hostname requires a second service entry, which allocates a different port. Since the app only listens on one port, the second hostname's proxy target is wrong. The workaround — a TCP port forwarder process — adds an unnecessary process, port allocation, and env var.

## Solution

Named aliases on a service. Each alias registers an additional proxy route pointing to the same allocated port, with full support for template expansion, CLI/dashboard display, and tunneling.

## Config

Add an `Aliases map[string]string` field to the `Service` struct:

```yaml
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
      admin: admin.approvethis
```

The key (`app`, `admin`) is a label used for template references. The value is the hostname (`.test` suffix optional in config, added during allocation — same as primary hostnames).

### Alias key rules

- Lowercase alphanumeric + hyphens (same regex as instance names)
- Used as template identifiers: `${web.alias.app}`, `${web.alias_url.app}`

### Validation

All rules applied identically to primary hostnames and aliases:

- Character regex: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$` (applied to stem, after stripping `.test`)
- Must contain project name
- Cannot be `outport.test`
- Globally unique across all primary hostnames and all aliases in all registered projects
- Cannot duplicate its own service's primary hostname
- A service must have a primary `hostname` to use `aliases` (aliases without a primary is a validation error)

### Error messages

| Condition | Message |
|---|---|
| Duplicate across projects | `service "web": alias "app" hostname "app.approvethis.test" is already registered by project "other/main"` |
| Duplicates own primary | `service "web": alias "app" hostname conflicts with service's own hostname` |
| Duplicates another alias | `service "web": alias "app" hostname "app.approvethis.test" conflicts with alias "app" on service "api"` |
| Invalid alias key | `service "web": alias key "App_1" is invalid (must be lowercase alphanumeric with hyphens)` |
| Fails project name check | `service "web": alias "app" hostname "app.foo.test" must contain project name "approvethis"` |
| Missing alias in template | `computed value "APP_URL": service "web" has no alias "app"` |

## Data Model

### Registry

New `Aliases` field on `Allocation`:

```go
type Allocation struct {
    ProjectDir            string                       `json:"project_dir"`
    Ports                 map[string]int               `json:"ports"`
    Hostnames             map[string]string            `json:"hostnames,omitempty"`
    Aliases               map[string]map[string]string `json:"aliases,omitempty"` // service -> alias name -> hostname
    EnvVars               map[string]string            `json:"env_vars,omitempty"`
    ApprovedExternalFiles []string                     `json:"approved_external_files,omitempty"`
}
```

Example registry entry:

```json
{
  "ports": { "web": 14139 },
  "hostnames": { "web": "approvethis.test" },
  "aliases": { "web": { "app": "app.approvethis.test", "admin": "admin.approvethis.test" } }
}
```

### Allocation

- `ComputeHostnames` remains focused on primary hostnames (unchanged)
- New `ComputeAliases` function handles alias hostname computation with the same instance-suffixing logic
- Instance suffixing applies to aliases identically to primaries (e.g., `app.approvethis.test` becomes `app.approvethis-bxcf.test` for instance `bxcf`)

### FindHostname

`registry.FindHostname` expands to search both `Hostnames` and `Aliases` maps for global uniqueness enforcement.

## Daemon Routing

### Route table

The route table structure changes from `map[string]int` to support Host header rewriting (needed for tunnels):

```go
type route struct {
    Port         int
    HostOverride string // empty for normal routes, set for tunnel routes
}

// RouteTable.routes becomes map[string]route
```

### BuildRoutes

Adds a second loop for aliases:

```go
// Primary hostnames (existing)
for svcName, hostname := range alloc.Hostnames {
    routes[hostname] = route{Port: alloc.Ports[svcName]}
}

// Aliases (new)
for svcName, aliases := range alloc.Aliases {
    for _, hostname := range aliases {
        routes[hostname] = route{Port: alloc.Ports[svcName]}
    }
}
```

### Proxy

When `route.HostOverride` is set, the proxy rewrites the Host header before forwarding:

```go
if route.HostOverride != "" {
    r.Host = route.HostOverride
}
```

This is used exclusively for tunnel routes (see Tunnels section). Normal `.test` routes have an empty `HostOverride`.

Lookup, TLS cert generation, and WebSocket support are unchanged — they already work with any hostname string.

## Template Expansion

Two new template fields per alias:

| Variable | Resolves to |
|---|---|
| `${service.hostname}` | Primary hostname (unchanged) |
| `${service.url}` | Primary URL (unchanged) |
| `${service.alias.NAME}` | Alias hostname (e.g., `app.approvethis.test`) |
| `${service.alias_url.NAME}` | Alias URL (e.g., `https://app.approvethis.test`) |

`BuildTemplateVars` adds alias variables alongside existing primary variables. The `validFields` map in config expands to recognize `alias` and `alias_url` as valid field prefixes.

The template parser must be extended to handle compound field lookups (`alias.NAME`, `alias_url.NAME`) since the current parser only handles single-level fields like `hostname` and `url`. The dot after `alias`/`alias_url` is a field separator, not part of the field name — so `${web.alias.app}` parses as service=`web`, field=`alias`, subfield=`app`.

Referencing a non-existent alias (e.g., `${web.alias.foo}` when `foo` isn't defined) is a config validation error.

During `outport share`, alias URL variables resolve to their respective tunnel URLs (same as primary URLs).

## Env Files & Computed Values

Aliases don't get automatic env vars. The primary `env_var` (e.g., `PORT=14139`) is unchanged. Alias hostnames and URLs are accessible through computed values:

```yaml
computed_values:
  APP_URL:
    value: "${web.alias_url.app}"
    env_files: [.env]
```

Writes `APP_URL=https://app.approvethis.test` to `.env`.

During `outport share`, computed values referencing alias URLs are rewritten to tunnel URLs. They revert when sharing stops.

## CLI Display

All hostnames shown as equal entry points, primary first, each on its own clickable line:

```
web (PORT=14139)
  https://approvethis.test
  https://app.approvethis.test
  https://admin.approvethis.test
```

### JSON output

Aliases appear in an `aliases` object:

```json
{
  "web": {
    "port": 14139,
    "env_var": "PORT",
    "hostname": "approvethis.test",
    "url": "https://approvethis.test",
    "aliases": {
      "app": { "hostname": "app.approvethis.test", "url": "https://app.approvethis.test" },
      "admin": { "hostname": "admin.approvethis.test", "url": "https://admin.approvethis.test" }
    }
  }
}
```

## Dashboard

Service cards show all URLs as clickable links — primary first, then aliases. The `/api/status` endpoint includes the same `aliases` structure as CLI JSON output.

## Tunnels

### Architecture change

All tunnels route through `localhost:80` (the outport proxy) instead of directly to service ports. This enables Host header rewriting so apps with host-based routing receive the correct `.test` hostname.

### Flow

```
Browser -> abc123.trycloudflare.com (marketing)
        -> cloudflared -> localhost:80 (outport proxy)
        -> proxy rewrites Host to approvethis.test -> Rails on :14139

Browser -> def456.trycloudflare.com (app)
        -> cloudflared -> localhost:80 (outport proxy)
        -> proxy rewrites Host to app.approvethis.test -> Rails on :14139
```

### Tunnel route registration

When `outport share` starts and a tunnel URL is assigned, a temporary route is added:

```go
routes["abc123.trycloudflare.com"] = route{Port: 14139, HostOverride: "approvethis.test"}
```

These routes are removed when sharing stops.

### Per-hostname tunnels

Each hostname (primary + aliases) gets its own cloudflared process. A service with a primary and two aliases spawns three cloudflared processes.

### Global cap

A `max_tunnels` setting in `~/.config/outport/config` caps concurrent cloudflared processes:

```ini
[tunnels]
max = 8
```

Default: 8. When the cap is reached, primary hostnames are tunneled first (one per service), then aliases in config order. Skipped aliases produce a warning:

```
Warning: tunnel limit reached (8). Skipped aliases: admin.approvethis.test
```

### Share output

```
Sharing 3 URLs:
  web  https://abc123.trycloudflare.com -> approvethis.test
  web  https://def456.trycloudflare.com -> app.approvethis.test
  web  https://ghi789.trycloudflare.com -> admin.approvethis.test
```

## Cleanup

`outport down` removes alias hostnames from the registry along with everything else. The fenced env file block is removed entirely (existing behavior), cleaning up any computed alias values. The daemon rebuilds routes from the registry, so alias routes disappear automatically.

No orphan concerns — aliases live entirely within the existing allocation lifecycle.

## Doctor

No new doctor checks needed. Alias validation is fully covered at config load time. Existing doctor checks (daemon running, DNS resolving, CA trusted) apply equally to alias hostnames.

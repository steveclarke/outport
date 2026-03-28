# Design: Custom `open` service list

## Summary

Add a top-level `open` field to `outport.yml` that declares which services `outport open` opens by default. Without this field, `outport open` continues to open all services with hostnames (today's behavior). With the field, only the listed services are opened.

## Config shape

```yaml
name: myapp

open:
  - web
  - frontend

services:
  web:
    hostname: myapp
    env_var: PORT
  frontend:
    hostname: app.myapp
    env_var: VITE_PORT
  admin:
    hostname: admin.myapp
    env_var: ADMIN_PORT
```

`open` is a YAML sequence of service names. Order determines browser tab order.

## Behavior

| Command | `open` field absent | `open` field present |
|---|---|---|
| `outport open` | Opens all services with hostnames | Opens only listed services |
| `outport open admin` | Opens `admin` | Opens `admin` (explicit name always works) |

The explicit `outport open <service>` path is unchanged — it bypasses the `open` list entirely.

## Validation

During `config.Load()`, after services are normalized:

1. Each entry in `open` must reference a service defined in `services`.
2. Each referenced service must have a `hostname`.
3. Duplicate entries are rejected.

Validation errors are descriptive, e.g.:
- `open: service "foo" does not exist in services`
- `open: service "redis" has no hostname`
- `open: duplicate entry "web"`

## Local overrides

`outport.local.yml` can declare `open`. When present, it replaces the base list entirely (not merged). This matches the existing override semantics for map fields like `aliases`.

The merge happens in `mergeLocal()` — if the local file has a non-nil `open` field, it replaces the base config's `open`.

## Implementation scope

### Config package (`internal/config/`)
- Add `Open []string` field to `rawConfig` (YAML tag `open`)
- Add `Open []string` field to `Config`
- Populate during `normalize()`
- Add `mergeLocal()` handling: local `open` replaces base when non-nil
- Add validation in `validate()`: existence, hostname presence, duplicates

### Open command (`cmd/open.go`)
- When `cfg.Open` is non-nil and non-empty, iterate only over `cfg.Open` instead of all services
- When `cfg.Open` is nil, preserve current behavior (all services with hostnames)

### Tests
- Config loading: valid `open` list parsed correctly
- Validation: unknown service, missing hostname, duplicates all rejected
- Local override: replaces base list
- Open command: respects the list when present, falls back when absent

### Docs
- `docs/reference/configuration.md`: document the `open` field
- `docs/reference/commands.md`: note the config-driven default behavior

### Init preset
- No changes — the default template has a single `web` service, so `open` isn't needed. Users add it when they have multiple services.

# Derived Values

> Computed environment variables that reference allocated ports.

## Status: Proposal

Based on real-world findings from integrating Outport with Unio — a monorepo with a Rails backend, two Nuxt frontends, Docker services, and worktree-based development.

## The Problem

Outport writes port numbers. Applications consume URLs.

When Outport assigns `RAILS_PORT=24920`, the frontend doesn't need the number `24920` — it needs `http://localhost:24920/api/v1`. The backend doesn't need `MAIN_PORT=14139` — it needs `http://unio.localhost:14139` for its CORS allow-list.

Today, every project has to bridge this gap in its own framework-specific way — Ruby string interpolation in config classes, `process.env` template literals in Nuxt config, Python f-strings in Django settings. Every project solves the same problem differently.

## The Solution

Let `.outport.yml` express templates that reference allocated ports. Outport resolves them and writes finished values to `.env`.

```yaml
name: unio
services:
  rails:
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  main:
    env_var: MAIN_PORT
    protocol: http
    env_file: frontend/apps/main/.env
  portal:
    env_var: PORTAL_PORT
    protocol: http
    env_file: frontend/apps/portal/.env

derived:
  NUXT_API_BASE_URL:
    value: "http://localhost:${RAILS_PORT}/api/v1"
    env_file:
      - frontend/apps/main/.env
      - frontend/apps/portal/.env

  CORE_CORS_ORIGINS:
    value: "http://unio.localhost:${MAIN_PORT},http://portal.unio.localhost:${PORTAL_PORT}"
    env_file: backend/.env

  CORE_FRONTEND_URL:
    value: "http://unio.localhost:${MAIN_PORT}"
    env_file: backend/.env
```

After `outport register`, `frontend/apps/main/.env` contains:

```
MAIN_PORT=14139
NUXT_API_BASE_URL=http://localhost:24920/api/v1
```

The frontend reads `NUXT_API_BASE_URL` from its `.env` — no template logic, no framework-specific config surgery. It just works.

## How It Works

### Resolution

1. Outport allocates all ports (existing behavior, unchanged)
2. For each derived value, substitute `${VAR_NAME}` with the allocated port
3. Write the resolved string to the specified `.env` file(s)
4. Same `dotenv.Merge()` used for ports handles derived values too

### Template syntax

- `${VAR_NAME}` — references an allocated port by its `env_var` name
- Only service port variables can be referenced (not other derived values)
- Unresolved references are a config validation error

### What can be referenced

A derived value can reference any `env_var` from the `services` section. References use the env_var name, not the service name:

```yaml
services:
  rails:
    env_var: RAILS_PORT    # ← this is what you reference
    env_file: backend/.env

derived:
  API_URL:
    value: "http://localhost:${RAILS_PORT}/api"   # ← ${RAILS_PORT}
    env_file: frontend/.env
```

### What can NOT be referenced

- Other derived values (no chaining, no circular dependency risk)
- Arbitrary environment variables from the system
- Variables not declared in `services`

If we need derived-from-derived later, a topological sort would handle it. But that's future work — YAGNI.

## Config Validation

New validation rules:

- Every `${VAR}` in a derived value must match an `env_var` from `services`
- Derived value names must not collide with service `env_var` names
- Derived values must have `value` and at least one `env_file` (no default — derived values must be explicit about where they go)

Error examples:

```
config: derived value "API_URL" references "${BACKEND_PORT}" but no service has env_var "BACKEND_PORT"
config: derived value "RAILS_PORT" conflicts with service env_var "RAILS_PORT"
```

## Implementation

### Config changes

Add to the config structs:

```go
type DerivedValue struct {
    Value    string       `yaml:"value"`
    EnvFiles []string     `yaml:"-"`  // resolved from env_file field
}

type Config struct {
    Name     string
    Services map[string]Service
    Derived  map[string]DerivedValue  // new
}
```

### Resolution function

```go
func ResolveDerived(derived map[string]DerivedValue, ports map[string]int) map[string]string {
    resolved := make(map[string]string)
    for name, dv := range derived {
        value := dv.Value
        for varName, port := range ports {
            value = strings.ReplaceAll(value, "${"+varName+"}", strconv.Itoa(port))
        }
        resolved[name] = value
    }
    return resolved
}
```

### Register command changes

After allocating ports and before writing `.env` files, resolve derived values and merge them into the env file vars map:

```go
// Existing: build envFileVars from port allocations
// New: resolve derived values and add to envFileVars
resolvedDerived := ResolveDerived(cfg.Derived, ports)
for name, dv := range cfg.Derived {
    for _, envFile := range dv.EnvFiles {
        envFileVars[envFile][name] = resolvedDerived[name]
    }
}
```

### Output changes

Derived values should appear in the register output, grouped separately:

```
unio
    rails             RAILS_PORT            → 24920  http://localhost:24920
    main              MAIN_PORT             → 14139  http://localhost:14139
    portal            PORTAL_PORT           → 14140  http://localhost:14140
    postgres          DB_PORT               → 21536
    redis             REDIS_PORT            → 29454

    derived:
    NUXT_API_BASE_URL                       → http://localhost:24920/api/v1
    CORE_CORS_ORIGINS                       → http://unio.localhost:14139,...
    CORE_FRONTEND_URL                       → http://unio.localhost:14139

Ports written to backend/.env, frontend/apps/main/.env, frontend/apps/portal/.env
```

### JSON output

```json
{
  "project": "unio",
  "instance": "main",
  "services": { ... },
  "derived": {
    "NUXT_API_BASE_URL": {
      "value": "http://localhost:24920/api/v1",
      "env_files": ["frontend/apps/main/.env", "frontend/apps/portal/.env"]
    }
  }
}
```

## What This Doesn't Do

- **No framework plugins.** Outport writes `.env` files. How your framework reads them is your problem. Most frameworks already read `.env` natively.
- **No runtime resolution.** Templates are resolved at `outport register` time, not at app boot time. The `.env` file contains finished strings.
- **No system env var references.** `${HOME}` or `${USER}` won't work — only Outport-managed port variables.
- **No derived-from-derived chaining.** A derived value can only reference service ports, not other derived values.

## Why This Matters

Without derived values, Outport solves port allocation but pushes URL construction into every project. Each project does framework-specific work to bridge "I have a port number" to "I have the URL my app needs."

With derived values, Outport writes finished URLs. The project declares its service topology once in `.outport.yml` and everything works — regardless of framework, language, or how many services talk to each other.

This is also the foundation for future features:
- **`outport share`** (#16) would update derived values with tunnel URLs instead of `localhost`
- **DNS/proxy** (#13) would update derived values with `.test` hostnames
- **Multi-service orchestration** (#17) relies entirely on derived values to wire services together through tunnels

Derived values are the connective tissue between port allocation and the networking features in the roadmap.

## Scope

- Derived values can only reference service ports (not other derived values)
- Template syntax is `${VAR_NAME}` only (no expressions, no conditionals)
- Resolution happens once at register time
- Same `.env` merge behavior as ports — always overwrite declared variables
- Derived values appear in `outport register`, `outport ports`, and `outport status` output

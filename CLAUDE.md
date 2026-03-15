# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Outport?

Outport is a deterministic port manager for multi-project, multi-instance development. It allocates stable, non-conflicting ports for services (Rails, Postgres, Redis, etc.), assigns `.test` hostnames, and writes everything to `.env` files. Each project instance (main checkout, worktree, or feature branch) gets its own port and hostname allocations so they don't collide with each other.

## Development Commands

All tasks use the `justfile`:

```bash
just build            # Compile to dist/outport
just test             # Run all tests (verbose)
just test-short       # Run tests (compact output)
just lint             # Run golangci-lint
just install          # Install to $GOPATH/bin
just run <args>       # Build and run (e.g., just run apply)
just release-dry-run  # Test GoReleaser locally
```

To run a single test: `go test ./internal/allocator/ -run TestHashPort -v`

## Architecture

Entry point: `main.go` → `cmd.Execute()` (Cobra CLI).

### Core packages (`internal/`)

- **allocator** — Port allocation via FNV-32a hash on `"{project}/{instance}/{service}"`. An optional preferred_port can be specified per service; when omitted, the hash is the primary allocation method. Port range: 10000–39999 (port 15353 reserved for daemon DNS). Collisions resolved by linear probing with wraparound.
- **registry** — Persistent JSON store at `~/.config/outport/registry.json`. Keys are `"{project}/{instance}"` (e.g., `"myapp/main"`, `"myapp/bxcf"`). Each allocation stores ports, hostnames, and protocols. Atomic writes via temp file + rename. Supports lookup by directory (`FindByDir`) and by project name (`FindByProject`).
- **config** — Loads/validates `.outport.yml`. Supports per-service env_file (string or array), preferred_port, protocol, hostname, and derived values (`${service.field}` and `${service.field:modifier}` templates with optional per-file overrides). `FindDir()` walks up from the current directory to locate the config. Validates env_var uniqueness per file, hostname format (must contain project name, requires http/https protocol), and derived value reference validity (service name + field + modifier).
- **instance** — Resolves instance names for projects. First instance of a project is "main". Additional instances get random 4-character consonant codes (e.g., "bxcf"). Looks up the registry by directory to find existing instances. Provides name validation (lowercase alphanumeric + hyphens).
- **daemon** — Long-running process providing DNS server (port 15353, resolves `*.test` to 127.0.0.1) and HTTP reverse proxy (port 80, routes requests by Host header to the correct service port). Watches the registry file for changes and rebuilds the route table automatically. Supports WebSocket proxying.
- **platform** — macOS-specific integration for the daemon. Manages the LaunchAgent plist (`~/Library/LaunchAgents/`) and `/etc/resolver/test` file for `.test` domain resolution. Provides setup/teardown/load/unload operations.
- **dotenv** — Writes allocated ports and derived values into a fenced block (`# --- begin outport.dev ---` / `# --- end outport.dev ---`) at the bottom of `.env` files. User content outside the block is preserved. Managed vars in the user section are removed and relocated into the block. Also provides `RemoveBlock()` for cleanup.
- **ui** — Lipgloss terminal styling constants.

### CLI commands (`cmd/`)

- **apply** — Main workflow: load config → resolve instance → load registry → allocate ports → compute hostnames → check hostname uniqueness → resolve derived values → merge `.env` → display results. Use `--force` to re-allocate all ports from scratch.
- **unapply** — Reverse of apply: clean managed blocks from all `.env` files and remove the project/instance from the registry.
- **init** — Creates a commented `.outport.yml` template in the current directory.
- **ports** — Show current project's allocated ports.
- **open** — Open HTTP/HTTPS services in the default browser. Requires `protocol: http` on services.
- **status** — Show all registered projects across the system. Prompts to remove stale entries interactively.
- **gc** — Remove stale registry entries where the project directory no longer exists.
- **rename** — Rename an instance of the current project. Updates hostnames and re-merges `.env` files.
- **promote** — Promote the current instance to "main". Demotes the existing main instance to a generated code name. Updates hostnames for both instances.
- **setup** — Install the `.test` DNS resolver and LaunchAgent daemon (macOS). Requires sudo for `/etc/resolver/test`.
- **teardown** — Remove the DNS resolver and daemon. Reverse of setup.
- **up** — Start the daemon (load the LaunchAgent).
- **down** — Stop the daemon (unload the LaunchAgent).
- **daemon** — (hidden) Run the DNS and proxy daemon directly. Invoked by launchd, not by users.

All commands support `--json` for machine-readable output. Each command has paired `print*Styled()` and `print*JSON()` output functions.

## Key Design Decisions

- **Stateless commands** — Each command independently loads config, resolves the instance, and loads the registry. No shared state between commands.
- **Deterministic allocation** — Same inputs always produce the same port (idempotent `outport apply`).
- **Instance model** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated 4-character codes. Instances can be renamed (`outport rename`) or promoted to main (`outport promote`). The registry is the source of truth for instance identity — directories are looked up by path.
- **`.test` hostnames** — Services with `hostname` + `protocol: http/https` get `.test` domain hostnames (e.g., `myapp.test`). Non-main instances get suffixed hostnames (e.g., `myapp-bxcf.test`). Hostnames are globally unique across all registered projects.
- **Template modifiers** — Derived values support `${service.field}` references. The `url` field resolves to the `.test` hostname URL (e.g., `http://myapp.test`). The `:direct` modifier gives the localhost URL with port (e.g., `http://localhost:3000`). Syntax: `${service.url}` vs `${service.url:direct}`.
- **Fenced .env blocks** — Managed variables are written in a `# --- begin/end outport.dev ---` fenced section. User content outside the block is never touched. Vars claimed by Outport are removed from the user section and relocated into the block.
- **Daemon architecture** — A LaunchAgent runs a DNS server (port 15353, `*.test` → 127.0.0.1) and HTTP reverse proxy (port 80, routes by Host header). The daemon watches the registry file and rebuilds routes on changes.
- **Error wrapping** — Uses `fmt.Errorf("context: %w", err)` throughout.

## Testing

**IMPORTANT: Every new feature or bug fix MUST include tests. Do not commit code without corresponding test coverage.**

Tests use table-driven patterns and `t.TempDir()` for filesystem isolation. No mocks — tests exercise real file I/O against temp directories. Run with `just test` (colored output via gotestsum).

## Release

GoReleaser builds for macOS + Linux (amd64 + arm64). Version injected via ldflags: `-X github.com/outport-app/outport/cmd.version={{.Version}}`. Releases triggered by pushing `v*` tags. Publishes to Homebrew tap `steveclarke/homebrew-tap`. See `project/releasing.md` for the full process.

## Git Conventions

- **Conventional commits** — Use prefixes: `feat:`, `fix:`, `chore:`, `test:`, `docs:`. GoReleaser's changelog excludes `docs:`, `chore:`, and `test:` commits.
- **Squash merge PRs** — One commit per feature/fix on master.
- **Link PRs to issues** — Use `Closes #N` in PR body.
- **Don't commit without explicit permission** from the user.

## Finalize Checklist

Run before committing or merging:

- [ ] `just lint` passes
- [ ] `just test` passes
- [ ] README.md commands list matches actual commands in `cmd/`
- [ ] `init` presets in `cmd/init.go` include any new service types
- [ ] `--json` output works for any changed commands
- [ ] CLAUDE.md reflects any architectural changes (new packages, commands, design decisions)

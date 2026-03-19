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
just run <args>       # Build and run (e.g., just run up)
just release-dry-run  # Test GoReleaser locally
```

To run a single test: `go test ./internal/allocator/ -run TestHashPort -v`

## Architecture

Entry point: `main.go` → `cmd.Execute()` (Cobra CLI).

### Core packages (`internal/`)

- **allocator** — Port allocation via FNV-32a hash on `"{project}/{instance}/{service}"`. An optional preferred_port can be specified per service; when omitted, the hash is the primary allocation method. Port range: 10000–39999 (port 15353 reserved for daemon DNS). Collisions resolved by linear probing with wraparound.
- **certmanager** — Local CA and server certificate lifecycle. Generates a CA (EC P-256, 10-year) during `outport system start`, creates per-hostname server certs on demand via TLS SNI callback, caches to disk (`~/.cache/outport/certs/`) and memory. Exports path helpers (`CACertPath`, `CAKeyPath`, `CertCacheDir`) used by `up` and `open` to detect HTTPS availability.
- **registry** — Persistent JSON store at `~/.local/share/outport/registry.json`. Keys are `"{project}/{instance}"` (e.g., `"myapp/main"`, `"myapp/bxcf"`). Each allocation stores ports, hostnames, and protocols. Atomic writes via temp file + rename. Supports lookup by directory (`FindByDir`) and by project name (`FindByProject`).
- **config** — Loads/validates `.outport.yml`. Supports per-service env_file (string or array), preferred_port, protocol, hostname, and computed values with bash-style parameter expansion (`${service.field}`, `${service.field:modifier}`, `${var:-default}`, `${var:+replacement}`). The `${instance}` variable is empty for main instances and set to the instance code for worktrees. `FindDir()` walks up from the current directory to locate the config. Validates env_var uniqueness per file, hostname format (must contain project name, requires http/https protocol), and computed value reference validity.
- **instance** — Resolves instance names for projects. First instance of a project is "main". Additional instances get random 4-character consonant codes (e.g., "bxcf"). Looks up the registry by directory to find existing instances. Provides name validation (lowercase alphanumeric + hyphens).
- **daemon** — Long-running process providing DNS server (port 15353, resolves `*.test` to 127.0.0.1), HTTP reverse proxy (port 80, 307 redirect to HTTPS when CA exists), and TLS reverse proxy (port 443, SNI-based cert selection). Watches the registry file for changes and rebuilds the route table automatically. Supports WebSocket proxying.
- **platform** — macOS-specific integration for the daemon. Manages the LaunchAgent plist (`~/Library/LaunchAgents/`) and `/etc/resolver/test` file for `.test` domain resolution. Provides setup/uninstall/start/stop/restart operations and CA trust/untrust via macOS `security` CLI.
- **doctor** — Diagnostic checks for the `outport doctor` command. `Check`, `Result`, and `Runner` types. `SystemChecks()` returns checks for DNS, daemon, CA, registry, and cloudflared. `ProjectChecks()` returns checks for config validation, registry lookup, and port availability. Each check returns pass/warn/fail with a fix suggestion.
- **envpath** — Env file path classification and external file approval. `ClassifyEnvFiles` resolves paths through symlinks (`filepath.EvalSymlinks`) and classifies each as internal or external to the project directory. `ConfirmExternalFiles` handles the interactive approval prompt, auto-approve (`-y`), and non-interactive error. `ExternalPaths` filters to external-only paths.
- **dotenv** — Writes allocated ports and computed values into a fenced block (`# --- begin outport.dev ---` / `# --- end outport.dev ---`) at the bottom of `.env` files. User content outside the block is preserved. Managed vars in the user section are removed and relocated into the block. Also provides `RemoveBlock()` for cleanup.
- **tunnel** — Tunnel provider abstraction and concurrent manager. Provider interface allows swapping tunnel backends (Cloudflare, etc.) without changing command code. Manager starts/stops multiple tunnels with all-or-nothing semantics and configurable timeout.
- **tunnel/cloudflare** — Cloudflare quick tunnel provider. Shells out to `cloudflared tunnel --url`, parses tunnel URL from stderr output.
- **ui** — Lipgloss terminal styling constants.

### CLI commands (`cmd/`)

Project commands (top-level):

- **setup** — Interactive first-run system setup. Uses charmbracelet/huh for a branded confirm prompt asking whether to enable `.test` domains with HTTPS. If yes, delegates to `runSystemStart`. If no, prints a tip about enabling later. JSON mode delegates entirely to `system start`.
- **up** — Main workflow: load config → resolve instance → load registry → allocate ports → compute hostnames → check hostname uniqueness → resolve computed values → merge `.env` → display results. Use `--force` to re-allocate all ports from scratch.
- **down** — Reverse of up: clean managed blocks from all `.env` files and remove the project/instance from the registry.
- **init** — Creates a commented `.outport.yml` template in the current directory.
- **ports** — Show current project's allocated ports.
- **open** — Open HTTP/HTTPS services in the default browser. Requires `protocol: http` on services.
- **share** — Tunnel HTTP services to public URLs via Cloudflare quick tunnels. Shares all HTTP services by default, or specify service names. Requires `cloudflared` binary. Rewrites `.env` files so `${service.url}` computed values resolve to tunnel URLs (`${service.url:direct}` stays localhost). Reverts `.env` on exit. Blocks until Ctrl+C.
- **rename** — Rename an instance of the current project. Updates hostnames and re-merges `.env` files.
- **promote** — Promote the current instance to "main". Demotes the existing main instance to a generated code name. Updates hostnames for both instances.
- **doctor** — Diagnostic command that checks the health of all Outport infrastructure: DNS resolver, LaunchAgent daemon, CA certificates, registry, and project config (when `.outport.yml` is found). Reports pass/warn/fail for each check with actionable fix suggestions. Read-only — never modifies system state.

System commands (under `outport system`):

- **system start** — Install the `.test` DNS resolver, LaunchAgent daemon, and local CA for HTTPS. Auto-runs setup on first use. Requires sudo for `/etc/resolver/test`. Generates a CA certificate and adds it to the macOS login keychain trust store (GUI password prompt). Listens on ports 80 (HTTP->HTTPS redirect) and 443 (TLS proxy).
- **system stop** — Stop the daemon (unload the LaunchAgent).
- **system restart** — Re-write plist and restart the daemon.
- **system status** — Show all registered projects across the system. Marks stale entries with a hint to run `system gc`.
- **system gc** — Remove stale registry entries where the project directory no longer exists.
- **system uninstall** — Remove the DNS resolver, daemon, CA certificate, and cached server certs. Reverse of start.

Hidden:

- **daemon** — (hidden) Run the DNS and proxy daemon directly. Invoked by launchd, not by users.

All commands support `--json` for machine-readable output. Each command has paired `print*Styled()` and `print*JSON()` output functions.

## Key Design Decisions

- **Stateless commands** — Each command independently loads config, resolves the instance, and loads the registry. No shared state between commands.
- **Deterministic allocation** — Same inputs always produce the same port (idempotent `outport up`).
- **Instance model** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated 4-character codes. Instances can be renamed (`outport rename`) or promoted to main (`outport promote`). The registry is the source of truth for instance identity — directories are looked up by path.
- **`.test` hostnames** — Services with `hostname` + `protocol: http/https` get `.test` domain hostnames (e.g., `myapp.test`). Non-main instances get suffixed hostnames (e.g., `myapp-bxcf.test`). Hostnames are globally unique across all registered projects.
- **Template expansion** — Computed values use bash-style parameter expansion. Service fields: `${service.port}`, `${service.hostname}`, `${service.url}`, `${service.url:direct}`. Standalone variables: `${instance}` (empty for main, instance code for worktrees). Operators: `${var:-default}` (use default if empty), `${var:+replacement}` (use replacement if non-empty). Example: `"myapp${instance:+-${instance}}"` → `myapp` for main, `myapp-xbjf` for worktrees.
- **Fenced .env blocks** — Managed variables are written in a `# --- begin/end outport.dev ---` fenced section. User content outside the block is never touched. Vars claimed by Outport are removed from the user section and relocated into the block.
- **Daemon architecture** — A LaunchAgent runs a DNS server (port 15353, `*.test` -> 127.0.0.1), HTTP reverse proxy (port 80), and TLS reverse proxy (port 443). When the CA is installed, port 80 issues 307 redirects to HTTPS. The daemon watches the registry file and rebuilds routes on changes.
- **Automatic HTTPS** — When the CA is installed (after `outport system start`), all `.test` hostnames automatically get HTTPS. Port 80 redirects to HTTPS via 307. Port 443 terminates TLS and proxies to the backend over plain HTTP. `${service.url}` produces `https://` URLs when the CA exists. No per-service opt-in required.
- **Command structure** — Project commands (`setup`, `up`, `down`, `init`, `ports`, `open`, `rename`, `promote`) are top-level. Machine-wide operations (`start`, `stop`, `restart`, `status`, `gc`, `uninstall`) live under `outport system`. `setup` is the recommended first-run command (interactive, delegates to `system start`). `up`/`down` follow the Docker Compose mental model (project-scoped).
- **XDG directory layout** — Registry at `~/.local/share/outport/registry.json`, CA at `~/.local/share/outport/`, cert cache at `~/.cache/outport/certs/`. `~/.config/outport/` reserved for future global config.
- **Error wrapping** — Uses `fmt.Errorf("context: %w", err)` throughout.
- **External env file safety** — Env file paths outside the project directory (where `.outport.yml` lives) require explicit developer approval. Paths are resolved through symlinks using `filepath.EvalSymlinks` before boundary checking. Approval can be interactive (prompt), auto (`-y` flag), or persisted (approved paths stored in registry allocation). All write commands (`up`, `down`, `rename`, `promote`, `share`) enforce this through `writeEnvFiles`/`removeEnvFiles` wrappers. A persistent warning is shown after every write that touches external files.

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
- [ ] Docs site (`docs/`) updated if commands, config fields, or user-facing behavior changed
- [ ] If docs changed: `npm run docs:build` succeeds and deploy via `npx wrangler pages deploy docs/.vitepress/dist --project-name outport-dev`

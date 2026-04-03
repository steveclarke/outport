## Development Commands

All tasks use the `justfile`:

```bash
just build            # Compile to dist/outport
just test             # Run all tests (verbose)
just test-short       # Run tests (compact output)
just test-e2e         # Run BATS end-to-end tests
just lint             # Run golangci-lint
just gosec            # Run gosec security scanner (source code)
just vulncheck        # Run govulncheck (dependencies)
just install          # Install to $GOPATH/bin
just run <args>       # Build and run (e.g., just run up)
just release-dry-run  # Test GoReleaser locally
just test-linux       # Run all tests on Linux via Docker
just dev-linux        # Shell into Linux dev container (starts if needed)
just dev-linux-down   # Stop Linux dev container
```

To run a single test: `go test ./internal/allocator/ -run TestHashPort -v`

## Architecture

Entry point: `main.go` → `cmd.Execute()` (Cobra CLI).

### Core packages (`internal/`)

- **allocation** — Builds registry allocations from config. Pure domain logic with no CLI dependencies.
- **allocator** — Port allocation via FNV-32a hash on `"{project}/{instance}/{service}"`. Port range: 10000–39999 (15353 reserved for daemon DNS). Collisions resolved by linear probing with wraparound.
- **certmanager** — Local CA and per-hostname server certificate lifecycle. Caches to `~/.cache/outport/certs/`.
- **registry** — Persistent JSON store at `~/.local/share/outport/registry.json`. Keys: `"{project}/{instance}"`. Atomic writes via temp file + rename.
- **config** — Loads/validates `outport.yml` (with optional `outport.local.yml` overrides). `FindDir()` walks up to locate config.
- **instance** — Resolves instance names. Validation: lowercase alphanumeric + hyphens.
- **daemon** — DNS server (port 15353), HTTP proxy (port 80), TLS proxy (port 443, SNI-based). Watches registry for route rebuilds. Serves dashboard at `outport.test`.
- **dashboard** — Embedded web dashboard (`go:embed`). JSON API, SSE live updates, health checker (configurable interval, only when clients connected).
- **platform** — OS-specific daemon lifecycle and trust. macOS: LaunchAgent plist, `/etc/resolver/test`, Keychain CA trust. Linux: systemd user service, systemd-resolved drop-in, distro-specific CA trust (`update-ca-certificates` / `update-ca-trust`), browser NSS trust via `certutil` (Chrome `~/.pki/nssdb`, Firefox profiles), Homebrew cert sync, `setcap` for privileged ports.
- **doctor** — Diagnostic checks returning pass/warn/fail with fix suggestions. On Linux, includes system resolver chain checks (resolv.conf routing, DNS stub listener, end-to-end resolution) via build-tagged `dns_linux.go`/`dns_other.go`.
- **portinfo** — System-level port scanning and process inspection via gopsutil (no shelling out). Discovers listening TCP ports, identifies processes, detects frameworks from project markers, and flags orphaned/zombie dev processes. `Lister` interface enables test injection.
- **envpath** — Env file path classification. Resolves symlinks before boundary checking.
- **dotenv** — Fenced `.env` block writer. Also provides `RemoveBlock()` for cleanup.
- **tunnel** — Provider abstraction + concurrent manager with all-or-nothing semantics.
- **tunnel/cloudflare** — Shells out to `cloudflared tunnel --url`, parses URL from stderr.
- **settings** — Global user settings from `~/.config/outport/config` (INI format, `go-ini/ini`). `Load()` returns defaults for missing file/keys. Consumers receive values as parameters — internal packages never import this.
- **ui** — Terminal color palette and text styles (lipgloss). `Init()` adapts to the terminal: respects `NO_COLOR` (strips all color, preserves bold), detects dark backgrounds (shifts grays brighter). Called once at CLI startup.

### CLI commands (`cmd/`)

Commands are defined in `cmd/*.go` — read them for details. Key conventions:

- All commands support `--json` for machine-readable output. Each command has paired `print*Styled()` and `print*JSON()` output functions. JSON output uses an envelope: `{"ok": true, "data": ..., "summary": "..."}` for success, `{"ok": false, "error": "...", "hint": "..."}` for errors. All JSON flows through `writeJSON()` / `writeJSONError()` in `cmd/cmdutil.go`.
- **daemon** is a hidden command invoked by launchd, not by users.

## Key Design Decisions

- **Stateless commands** — Each command independently loads config, resolves the instance, and loads the registry. No shared state between commands.
- **Deterministic allocation** — Same inputs always produce the same port (idempotent `outport up`).
- **Instance model** — The first checkout of a project is "main". Additional checkouts (worktrees, clones) get auto-generated 4-character codes. Instances can be renamed (`outport rename`) or promoted to main (`outport promote`). The registry is the source of truth for instance identity — directories are looked up by path.
- **`.test` hostnames** — Services declare `.test` hostnames explicitly (e.g., `hostname: myapp.test`). The `.test` suffix is required in config. Non-main instances get suffixed hostnames (e.g., `myapp-bxcf.test`). Hostnames are globally unique across all registered projects. Services can also define named `aliases` (a map of label → hostname) that register additional proxy routes to the same port — each alias follows the same validation rules as primary hostnames and gets the same instance suffix treatment for non-main instances.
- **`open` list** — Optional top-level `open` field in `outport.yml` lists which services `outport open` opens by default. When absent, all services with hostnames are opened (original behavior). Validated: each entry must exist and have a hostname. Overridable in `outport.local.yml` (replaces entirely).
- **Template expansion** — Computed values use bash-style parameter expansion. Service fields: `${service.port}`, `${service.hostname}`, `${service.url}`, `${service.url:direct}`, `${service.env_var}`, `${service.alias.NAME}` (alias hostname by label), `${service.alias_url.NAME}` (alias `https://` URL by label). Standalone variables: `${instance}` (empty for main, instance code for worktrees), `${project_name}` (project name from config). Operators: `${var:-default}` (use default if empty), `${var:+replacement}` (use replacement if non-empty). Example: `"${project_name}${instance:+-${instance}}"` → `myapp` for main, `myapp-xbjf` for worktrees.
- **Local config overrides** — `outport.local.yml` (not committed) merges on top of `outport.yml` at load time. Only services already in the base config can be overridden — field-level merge where non-zero/non-empty local values win. Aliases and the `open` list replace entirely (not merged). Project name and computed values in the local file are ignored. Validation runs on the merged result.
- **Fenced .env blocks** — Managed variables are written in a `# --- begin/end outport.dev ---` fenced section. User content outside the block is never touched. Vars claimed by Outport are removed from the user section and relocated into the block.
- **External env file safety** — Env file paths outside the project directory require explicit developer approval. Paths are resolved through symlinks using `filepath.EvalSymlinks` before boundary checking. All write commands enforce this through `writeEnvFiles`/`removeEnvFiles` wrappers.
- **Auto-restart on version mismatch** — Every CLI command (except `daemon`, `setup`, and `system` subcommands) checks the running daemon's version via `/api/status`. If they differ, the daemon is silently restarted. Best-effort — failures are silently ignored. Implementation in `cmd/version_check.go`.
- **Dashboard at `outport.test`** — The proxy handler intercepts this hostname before route lookup and delegates to the embedded dashboard handler. SSE for real-time updates. Config validation rejects `outport.test` as a project hostname.
- **Linux support** — The `internal/platform/` package provides Linux implementations via `//go:build linux`. systemd user services replace LaunchAgent, a systemd-resolved drop-in config (`/etc/systemd/resolved.conf.d/outport-test.conf`) replaces `/etc/resolver/test`, and `setcap CAP_NET_BIND_SERVICE` replaces launchd socket activation for privileged port binding. CA trust uses distro detection (Debian/Fedora/Arch/openSUSE paths, following mkcert's pattern).
- **Tunnel-through-proxy** — `outport share` routes all tunnels through the HTTP proxy (port 80) using Host header rewriting rather than tunneling directly to each service port. `cloudflared` connects to `http://localhost:80` for each hostname, and the proxy forwards to the correct service based on the Host header. This means both primary hostnames and aliases can be shared independently. The `tunnels.max` setting (default `8`) caps concurrent tunnel processes.
- **Global settings** — INI file at `~/.config/outport/config`. Settings: `dashboard.health_interval` (default `3s`), `dns.ttl` (default `60`), `tunnels.max` (default `8`), `network.interface` (default: auto-detect). `outport setup` creates the file with commented-out defaults. The daemon loads settings once at startup and passes values down as parameters. Proxy ports (80/443) are intentionally not configurable — DNS resolves `*.test` to an IP only, so non-standard ports break hostname access.

## Testing

**IMPORTANT: Every new feature or bug fix MUST include tests. Do not commit code without corresponding test coverage.**

Tests use table-driven patterns and `t.TempDir()` for filesystem isolation. No mocks — tests exercise real file I/O against temp directories. Run with `just test` (colored output via gotestsum).

## Release

Version injected via ldflags: `-X github.com/steveclarke/outport/cmd.version={{.Version}}`. Releases triggered by pushing `v*` tags. GoReleaser produces: tar.gz archives (with shell completions), `.deb`/`.rpm` Linux packages (with completions in system directories), and a Homebrew formula update to `steveclarke/homebrew-tap`. A `curl|sh` install script (`install.sh`) downloads from GitHub Releases with SHA-256 verification. Release process docs are in the private `backstage` repo.

## Git Conventions

- **Conventional commits** — Use prefixes: `feat:`, `fix:`, `chore:`, `test:`, `docs:`. GoReleaser's changelog excludes `docs:`, `chore:`, and `test:` commits.
- **Squash merge PRs** — One commit per feature/fix on master.
- **Link PRs to issues** — Use `Closes #N` in PR body.
- **Don't commit without explicit permission** from the user.

## Design Context

- **Users:** Solo developer managing multiple local projects. Primary job: find a `.test` URL and click it. Secondary: glance at service health.
- **Brand personality:** Reliable, clean, smart.
- **Aesthetic:** Polished product (not a dev utility dump). Reference: Docker Desktop containers view.
- **Colors:** Navy `#031C54` (headings), steel blue `#2E86AB` (links/accent), warm cream `#faf8f5` (background), `#f5f0e8` (soft bg), white (surface).
- **Fonts:** Barlow Bold (headings, tight letter-spacing), Inter (body), SF Mono/Fira Code (mono).
- **Principles:** URL-first, full-width no waste, on-brand, progressive disclosure, polished not utilitarian.
- **Full context:** See `.impeccable.md` in project root.

## Finalize Checklist

Run before committing or merging:

- [ ] `just lint` passes
- [ ] `just test` passes
- [ ] README.md commands list matches actual commands in `cmd/`
- [ ] `init` presets in `cmd/init.go` include any new service types
- [ ] `--json` output works for any changed commands
- [ ] CLAUDE.md reflects any architectural changes (new packages, commands, design decisions)
- [ ] Docs site (`docs/`) updated if commands, config fields, or user-facing behavior changed
- [ ] If docs changed: `bin/deploy-docs`
- [ ] VS Code extension — if config fields, commands, or user-facing behavior changed, check that the extension's JSON schema and `docs/guide/vscode.md` still match. Extension repo: `steveclarke/outport-vscode`

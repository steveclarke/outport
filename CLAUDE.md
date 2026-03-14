# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Outport?

Outport is a deterministic port manager for multi-project, multi-worktree development. It allocates stable, non-conflicting ports for services (Rails, Postgres, Redis, etc.) and writes them to `.env` files. Each git worktree gets its own port allocations so feature branches don't collide with each other.

## Development Commands

All tasks use the `justfile`:

```bash
just build            # Compile to dist/outport
just test             # Run all tests (verbose)
just test-short       # Run tests (compact output)
just lint             # Run golangci-lint
just install          # Install to $GOPATH/bin
just run <args>       # Build and run (e.g., just run register)
just release-dry-run  # Test GoReleaser locally
```

To run a single test: `go test ./internal/allocator/ -run TestHashPort -v`

## Architecture

Entry point: `main.go` → `cmd.Execute()` (Cobra CLI).

### Core packages (`internal/`)

- **allocator** — Port allocation via FNV-32a hash on `"{project}/{instance}/{service}"`. An optional preferred_port can be specified per service; when omitted, the hash is the primary allocation method. Port range: 10000–39999. Collisions resolved by linear probing with wraparound.
- **registry** — Persistent JSON store at `~/.config/outport/registry.json`. Keys are `"{project}/{instance}"` (e.g., `"myapp/main"`, `"myapp/feature-xyz"`). Atomic writes via temp file + rename.
- **config** — Loads/validates `.outport.yml`. Supports per-service env_file for writing to different `.env` files (string or array for multi-file writes), preferred_port, and explicit protocol. Validates env_var uniqueness per file.
- **worktree** — Detects git worktree vs. main checkout. Parses `.git` file to extract worktree name. Defaults to `"main"`.
- **dotenv** — Merges allocated ports into `.env` files. Variables declared in `.outport.yml` are always overwritten; all other lines are preserved untouched.
- **ui** — Lipgloss terminal styling constants.

### CLI commands (`cmd/`)

- **register** — Main workflow: load config → detect worktree → load registry → allocate ports → merge `.env` → display results. Use `--force` to re-allocate all ports from scratch.
- **unregister** — Remove the current project/worktree from the registry and clean managed variables from `.env` files.
- **init** — Interactive setup, creates `.outport.yml` with selected services.
- **ports** — Show current project's allocated ports.
- **open** — Open HTTP/HTTPS services in the default browser. Requires `protocol: http` on services.
- **status** — Show all registered projects across the system. Prompts to remove stale entries interactively.
- **gc** — Remove stale registry entries where the project directory no longer exists.

All commands support `--json` for machine-readable output. Each command has paired `print*Styled()` and `print*JSON()` output functions.

## Key Design Decisions

- **Stateless commands** — Each command independently loads config, worktree info, and registry. No shared state between commands.
- **Deterministic allocation** — Same inputs always produce the same port (idempotent `outport register`).
- **Instance = worktree name** — "main" for the primary checkout, worktree directory name for feature branches. Combined with project name to form unique registry keys.
- **Config-driven .env merge** — Variables in `.outport.yml` are always written to `.env`, overwriting existing values. All other lines (comments, unrelated variables) are preserved.
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

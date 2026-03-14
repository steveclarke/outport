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
just run <args>       # Build and run (e.g., just run up)
just release-dry-run  # Test GoReleaser locally
```

To run a single test: `go test ./internal/allocator/ -run TestHashPort -v`

## Architecture

Entry point: `main.go` → `cmd.Execute()` (Cobra CLI).

### Core packages (`internal/`)

- **allocator** — Port allocation: tries preferred_port first, falls back to FNV-32a hash on `"{project}/{instance}/{service}"`. Port range: 10000–39999. Collisions resolved by linear probing with wraparound.
- **registry** — Persistent JSON store at `~/.config/outport/registry.json`. Keys are `"{project}/{instance}"` (e.g., `"myapp/main"`, `"myapp/feature-xyz"`). Atomic writes via temp file + rename.
- **config** — Loads/validates `.outport.yml`. Supports flat services, groups (with shared env_file), per-service env_file (string or array for multi-file writes), preferred_port, and explicit protocol. Normalization flattens groups into a unified services map. Validates env_var uniqueness per file.
- **worktree** — Detects git worktree vs. main checkout. Parses `.git` file to extract worktree name. Defaults to `"main"`.
- **dotenv** — Merges allocated ports into `.env` files. Lines tagged with `" # managed by outport"` (note leading space before `#`) are Outport-managed; all other lines are preserved untouched.
- **ui** — Lipgloss terminal styling constants.

### CLI commands (`cmd/`)

- **up** — Main workflow: load config → detect worktree → load registry → allocate ports → merge `.env` → display results.
- **init** — Interactive setup, creates `.outport.yml` with selected services.
- **ports** — Show current project's allocated ports.
- **status** — Show all registered projects across the system.
- **gc** — Remove stale registry entries where the project directory no longer exists.

All commands support `--json` for machine-readable output. Each command has paired `print*Styled()` and `print*JSON()` output functions.

## Key Design Decisions

- **Stateless commands** — Each command independently loads config, worktree info, and registry. No shared state between commands.
- **Deterministic allocation** — Same inputs always produce the same port (idempotent `outport up`).
- **Instance = worktree name** — "main" for the primary checkout, worktree directory name for feature branches. Combined with project name to form unique registry keys.
- **Marker-based .env merge** — The `" # managed by outport"` suffix is the sole mechanism to distinguish managed vs. user-set variables. User variables are never overwritten.
- **Error wrapping** — Uses `fmt.Errorf("context: %w", err)` throughout.

## Testing

Tests use table-driven patterns and `t.TempDir()` for filesystem isolation. No mocks — tests exercise real file I/O against temp directories.

## Release

GoReleaser builds for macOS + Linux (amd64 + arm64). Version injected via ldflags: `-X github.com/outport-app/outport/cmd.version={{.Version}}`. Releases triggered by pushing `v*` tags. Publishes to Homebrew tap `steveclarke/homebrew-tap`.

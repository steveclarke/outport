# Commands

All commands support `--json` for machine-readable output.

## Core

### `outport init`

Create `.outport.yml` for this project.

```bash
outport init
```

Creates a commented template in the current directory. Does not allocate ports or modify the registry.

### `outport apply`

Allocate ports, assign hostnames, and write `.env` files. Alias: `outport a`.

```bash
outport apply
outport a          # short alias
outport apply --force  # re-allocate all ports from scratch
```

Reads `.outport.yml`, allocates deterministic ports, saves to the registry, and writes them to `.env`. Idempotent — running again reuses existing allocations.

| Flag | Description |
|------|-------------|
| `--force` | Ignore existing allocations and re-allocate all ports |
| `--json` | Output results as JSON |

### `outport unapply`

Remove ports and clean `.env` files.

```bash
outport unapply
```

Removes the managed block from all `.env` files and removes the project/instance from the registry.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON |

### `outport ports`

Show ports for the current project.

```bash
outport ports
outport ports --check    # check if ports are accepting connections
outport ports --derived  # include derived values
```

| Flag | Description |
|------|-------------|
| `--check` | Check if ports are accepting connections |
| `--derived` | Show derived values |
| `--json` | Output results as JSON |

## Navigation

### `outport open`

Open HTTP services in the browser.

```bash
outport open         # open all HTTP services
outport open web     # open a specific service
```

Opens services with `protocol: http` or `protocol: https` in your default browser. Works best with `.test` domains set up (`outport setup`).

### `outport status`

Show all registered projects and their ports.

```bash
outport status
outport status --check  # include port health checks
```

Lists every project/instance in the registry with their allocated ports. Prompts to remove stale entries interactively.

| Flag | Description |
|------|-------------|
| `--check` | Check if ports are accepting connections |
| `--json` | Output results as JSON |

## Maintenance

### `outport gc`

Remove stale entries from the registry.

```bash
outport gc
```

Scans the registry and removes entries whose project directories or config files no longer exist.

### `outport rename`

Rename an instance of the current project.

```bash
outport rename <old-name> <new-name>
```

Updates the instance name in the registry and regenerates hostnames in `.env` files.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON |

### `outport promote`

Promote the current instance to main.

```bash
outport promote
```

Promotes the current worktree instance to "main", demoting the existing main instance to an auto-generated code. Updates hostnames for both instances.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON |

## Daemon

These commands manage the `.test` domain DNS resolver, HTTPS reverse proxy, and local Certificate Authority.

### `outport setup`

Install the DNS resolver, daemon, and local Certificate Authority.

```bash
outport setup
```

Installs the `.test` DNS resolver (`/etc/resolver/test`, requires sudo), a LaunchAgent that runs a DNS server (port 15353) and reverse proxy (ports 80 and 443), and generates a local Certificate Authority that is added to the macOS trust store. After setup, `*.test` hostnames resolve to your local services with full HTTPS support. HTTP requests are automatically redirected to HTTPS via 307.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON (includes `ca_generated`, `ca_trusted` fields) |

### `outport teardown`

Remove the DNS resolver, daemon, and Certificate Authority.

```bash
outport teardown
```

Unloads the daemon, removes the LaunchAgent plist, removes the DNS resolver file, removes the CA from the macOS trust store, deletes the CA files, and removes cached certificates from `~/.cache/outport/certs/`.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON (includes `ca_removed`, `certs_cleaned` fields) |

### `outport up`

Start the daemon.

```bash
outport up
```

Loads the LaunchAgent to start the DNS resolver and reverse proxy.

### `outport down`

Stop the daemon.

```bash
outport down
```

Unloads the LaunchAgent to stop the DNS resolver and reverse proxy.

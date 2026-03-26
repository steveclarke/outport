---
description: All Outport CLI commands — setup, init, up, down, ports, open, share, rename, promote, doctor, and system management.
---

# Commands

Outport commands fall into two groups: **project commands** that operate on the current directory's `outport.yml`, and **system commands** that manage machine-wide infrastructure like DNS, HTTPS, and the daemon. All commands support `--json` for machine-readable output.

Port allocations are stored in the **registry** — a JSON file at `~/.local/share/outport/registry.json` that maps every project and instance to its allocated ports, hostnames, and env vars. The registry is what makes allocations persistent and deterministic across runs.

## Project Commands

These commands operate on the current project (the directory containing `outport.yml`).

### `outport init`

Create an `outport.yml` in the current directory. This marks the directory as an Outport project — all commands run from this directory or any subdirectory will use this config.

```bash
outport init
```

Creates a commented template in the current directory. This is a config-only step — it does not allocate ports or register the project.

### `outport up`

Allocate ports, assign hostnames, and write env files.

```bash
outport up
outport up --force  # re-allocate all ports from scratch
```

Reads `outport.yml`, preserves existing port allocations from the registry, allocates ports for any new services, and writes everything to your env files (`.env` by default, configurable per service via `env_file`). Safe to run repeatedly — existing ports stay the same.

| Flag | Description |
|------|-------------|
| `--force` | Discard existing allocations and re-allocate all ports from scratch |
| `--yes`, `-y` | Auto-approve writing env files outside the project directory |
| `--json` | Output results as JSON |

### `outport down`

Remove ports and clean env files.

```bash
outport down
```

Removes the managed block from all env files and removes the project/instance from the registry.

| Flag | Description |
|------|-------------|
| `--yes`, `-y` | Auto-approve removing env files outside the project directory |
| `--json` | Output results as JSON |

### `outport ports`

Show ports for the current project.

```bash
outport ports
outport ports --check    # check if ports are accepting connections
outport ports --computed  # include computed values
```

| Flag | Description |
|------|-------------|
| `--check` | Check if ports are accepting connections |
| `--computed` | Show computed values |
| `--json` | Output results as JSON |

### `outport open`

Open HTTP services in the browser.

```bash
outport open         # open all HTTP services
outport open web     # open a specific service
```

Opens HTTP services (those with a `hostname`) in your default browser. Works best with `.test` domains set up (`outport system start`).

### `outport qr`

Show QR codes for accessing HTTP services from mobile devices.

```bash
outport qr              # LAN QR codes for all HTTP services
outport qr web          # QR code for a specific service
outport qr --tunnel     # tunnel URL QR codes (requires active outport share)
```

Displays a scannable QR code encoding a LAN URL (`http://<your-ip>:<port>`) for each HTTP service. Scan with your phone on the same Wi-Fi network to open the dev app. Use `--tunnel` to show Cloudflare tunnel URLs instead (requires `outport share` running in another terminal).

If the service appears to be bound to localhost only, a hint is shown suggesting to bind to `0.0.0.0`.

| Flag | Description |
|------|-------------|
| `--tunnel` | Show tunnel URL instead of LAN URL |
| `--interface` | Override auto-detected network interface (e.g., `en0`). Outport scans your network interfaces to find your LAN IP — if it picks the wrong one (e.g., VPN adapter instead of Wi-Fi), use this to specify the correct interface. |
| `--json` | Output URLs as JSON |

### `outport share`

Tunnel HTTP services to public URLs via Cloudflare quick tunnels.

```bash
outport share              # tunnel all HTTP services
outport share web          # tunnel a specific service
outport share web vite     # tunnel specific services
```

Creates temporary public URLs for HTTP services (those with a `hostname`). Requires `cloudflared` (`brew install cloudflared`). The command blocks until you press Ctrl+C.

While sharing, env files are rewritten so computed values using `${service.url}` resolve to the tunnel URLs. This means CORS origins, API base URLs, and other computed values automatically point to the public tunnel URLs. Values using `${service.url:direct}` stay as localhost. On exit, env files revert to local URLs. Restart your services after starting and stopping `outport share`.

| Flag | Description |
|------|-------------|
| `--json` | Output tunnel URLs as JSON |

### `outport rename`

Rename an instance of the current project.

```bash
outport rename [old-name] <new-name>
```

If `old-name` is omitted, renames the current directory's instance. Updates the instance name in the registry and regenerates hostnames in env files.

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

## System Commands

These commands manage machine-wide infrastructure: the `.test` domain DNS resolver, HTTPS reverse proxy, local Certificate Authority, and registry maintenance.

### `outport setup`

Interactive first-run system setup.

```bash
outport setup
```

Guides you through enabling `.test` domains with HTTPS. The `.test` domain setup is optional — without it, `outport up` still works for deterministic ports and env files. Also creates the [global settings](/reference/configuration#global-settings) file at `~/.config/outport/config` if it doesn't exist.

| Flag | Description |
|------|-------------|
| `--json` | Non-interactive, runs full setup |

### `outport system start`

Install the daemon, DNS resolver, and local Certificate Authority.

```bash
outport system start
```

::: tip First-time setup?
Use `outport setup` instead — it provides a guided interactive experience. `outport system start` is for machines where setup has already been completed.
:::

On first run, installs the `.test` domain infrastructure (requires sudo) and generates a local Certificate Authority. On subsequent runs, starts the daemon if it is not already running. See [How It Works](/guide/how-it-works) for details on what the daemon does.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON (includes `ca_generated`, `ca_trusted` fields) |

### `outport system stop`

Stop the daemon.

```bash
outport system stop
```

Stops the daemon service, shutting down the DNS resolver and reverse proxy.

### `outport system restart`

Re-write the daemon service configuration and restart.

```bash
outport system restart
```

Useful after upgrading Outport to pick up the new binary path.

### `outport system status`

Show all registered projects and their ports.

```bash
outport system status
outport system status --check  # include port health checks
```

Lists every project/instance in the registry with their allocated ports. Prompts to remove stale entries interactively.

| Flag | Description |
|------|-------------|
| `--check` | Check if ports are accepting connections |
| `--json` | Output results as JSON |

### `outport system prune`

Remove stale entries from the registry.

```bash
outport system prune
```

Scans the registry and removes entries whose project directories or config files no longer exist.

### `outport system uninstall`

Remove the DNS resolver, daemon, and Certificate Authority.

```bash
outport system uninstall
```

Stops the daemon, removes the service configuration, removes the DNS resolver config, removes the CA from the system trust store, deletes the CA files, and removes cached certificates from `~/.cache/outport/certs/`.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON (includes `ca_removed`, `certs_cleaned` fields) |

### `outport doctor`

Diagnose issues with the Outport system.

```bash
outport doctor
```

Runs diagnostic checks on all Outport infrastructure and project configuration. Reports pass/warn/fail for each check with actionable fix suggestions. Checks include:

**System checks** (always run): DNS resolver config, resolver content, daemon service file, service binary validity, daemon status, DNS resolution, HTTP proxy (port 80), HTTPS proxy (port 443), CA certificate and key existence, CA expiry, CA trust, registry validity, cloudflared availability.

**Project checks** (when `outport.yml` found): config file validation, project registration in the registry, per-service port status (running or not — both are informational, not failures).

Exit code 0 if all checks pass or warn. Exit code 1 if any check fails.

| Flag | Description |
|------|-------------|
| `--json` | Output results as JSON (includes `results` array and `passed` boolean) |

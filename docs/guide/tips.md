---
description: Tips for getting the most out of Outport, and troubleshooting common issues with DNS, the daemon, ports, and worktrees.
---

# Tips & Troubleshooting

## Tips

### Pin the dashboard

Pin [https://outport.test](https://outport.test) in your browser for a live view of all your projects and services. It shows which services are up or down and gives you clickable links to every `.test` URL. The dashboard updates automatically — no need to refresh. See the [Dashboard guide](/guide/dashboard) for details.

### Multiple worktrees of the same project

Here's a full walkthrough. Say you have a project checked out at `~/src/myapp` and want a second checkout for a feature branch:

```bash
# Your main checkout already has Outport configured
cd ~/src/myapp
outport up               # myapp [main] → myapp.test

# Create a worktree for a feature branch
git worktree add ../myapp-feature feature-branch

# Move into the worktree and run outport up
cd ../myapp-feature
outport up               # myapp [xbjf] → myapp-xbjf.test
```

Outport detects that the project name `myapp` is already registered and assigns a short 4-character code to the new checkout. Each worktree gets its own ports, hostnames, and `.env` — you can run both simultaneously without conflicts.

To give a worktree a readable name (run from inside the worktree):

```bash
outport rename feature
# Now: myapp-feature.test
```

To make a worktree the primary checkout:

```bash
outport promote
# This worktree becomes [main], the old main gets a generated code
```

### Docker Compose conflicts with worktrees

If `docker compose up` from one worktree replaces another's containers, add a `COMPOSE_PROJECT_NAME` computed value:

```yaml
computed:
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: .env
```

This gives each instance a unique Docker Compose project name.

### Per-machine config overrides

Create `outport.local.yml` (gitignored) in the same directory as your `outport.yml` to override service fields per-machine. Useful when your setup differs from the team's defaults:

```yaml
# outport.local.yml
services:
  postgres:
    preferred_port: 5432    # use system Postgres
```

See [Local Overrides](/reference/configuration#local-overrides-outport-local-yml) for full details.

### Adding Outport to a project setup script

Make it optional so developers without Outport aren't blocked:

```bash
if command -v outport > /dev/null 2>&1; then
  outport up
else
  echo "Outport not found — install: brew install steveclarke/tap/outport"
fi
```

## Troubleshooting

### Start with outport doctor

If something isn't working, start here:

```bash
outport doctor
```

This checks DNS, the daemon, TLS certificates, the registry, and (if you're in a project directory) your `outport.yml` and port status. Each check shows pass/fail with a fix suggestion.

Common results and what they mean:

- **DNS resolver: FAIL** — the resolver file is missing or misconfigured. Run `outport system start` to reinstall it.
- **Daemon: FAIL** — the background service isn't running. Run `outport system start`.
- **CA trust: FAIL** — the local certificate authority isn't trusted by your OS. Run `outport system start` to re-trust it.
- **Port check: not listening** — this is informational, not a failure. It means the service isn't running on that port yet. Start your services and it will pass.

### Daemon not running

If `outport up` shows this hint:

```
Hint: The outport daemon is not running.
Run 'outport system start' to enable .test domains.
```

Port allocation and `.env` writing still work without the daemon — you just won't have `.test` hostnames or HTTPS. Run `outport system start` to enable them.

### After upgrading Outport

After `brew upgrade outport`, bounce the daemon to pick up the new binary:

```bash
outport system restart
```

This re-writes the daemon service configuration with the new binary path and restarts the daemon.

### Ports changed unexpectedly

Outport allocates deterministic ports — the same project, instance, and service name always produce the same port. If your ports changed, one of these happened:

- **You re-registered from a different directory** — the instance name may have changed
- **You used `--force`** — this re-allocates all ports from scratch
- **Another project claimed your preferred port** — preferred ports are first-come-first-served across all registered projects

Run `outport status` to see your current allocations and `outport system status` to see all registered projects.

### Stale registry entries

If `outport system status` shows stale entries (projects whose directories no longer exist):

```bash
outport system prune
```

This removes entries where the project directory or `outport.yml` is missing.

### About sudo and system changes

`outport setup` and `outport system start` ask for your password to do three things: install a DNS resolver file so `.test` domains work, install a background service (daemon), and trust a locally-generated certificate authority so HTTPS works without browser warnings. These are standard operations — the same things tools like mkcert and dnsmasq do.

Everything is reversible: `outport system uninstall` removes all of it cleanly. Outport never modifies `/etc/hosts`, never touches your existing certificates, and the local CA is only trusted on your machine.

### Port 80 or 443 already in use

If `outport system start` fails with "port 80 is already in use", another server (nginx, Apache, another dev tool) is using that port. Stop it first, then retry.

Find what's using the port:

```bash
# macOS
sudo lsof -iTCP:80 -sTCP:LISTEN

# Linux
sudo ss -tlnp 'sport = 80'
```

### What if I delete the registry?

If `~/.local/share/outport/registry.json` gets deleted or corrupted, Outport will recreate it the next time you run `outport up`. Because port allocation is deterministic (based on project name + instance + service name), you'll get the same ports back. Just run `outport up` in each project directory to re-register.

### Clean reinstall

To start completely fresh:

```bash
outport system uninstall    # removes daemon, DNS, certs, and registry
outport system start        # reinstall from scratch
```

Then re-register each project with `outport up`. Your `outport.yml` files are untouched — they live in your project directories, not in the system config.

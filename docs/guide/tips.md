---
description: Tips for getting the most out of Outport, and troubleshooting common issues with DNS, the daemon, ports, and worktrees.
---

# Tips & Troubleshooting

## Tips

### Pin the dashboard

Pin [https://outport.test](https://outport.test) in your browser for a live view of all your projects and services. It shows which services are up or down and gives you clickable links to every `.test` URL. The dashboard updates automatically — no need to refresh. See the [Dashboard guide](/guide/dashboard) for details.

### Multiple worktrees of the same project

Each worktree gets its own instance with unique ports and suffixed hostnames:

```bash
cd ~/src/myapp           # main checkout
outport up               # myapp [main] → myapp.test

cd ~/src/myapp-feature   # worktree
outport up               # myapp [xbjf] → myapp-xbjf.test
```

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

### Port 80 or 443 already in use

If `outport system start` fails with "port 80 is already in use", another server (nginx, Apache, another dev tool) is using that port. Stop it first, then retry.

Find what's using the port:

```bash
# macOS
sudo lsof -iTCP:80 -sTCP:LISTEN

# Linux
sudo ss -tlnp 'sport = 80'
```

### Clean reinstall

To start completely fresh:

```bash
outport system uninstall    # removes daemon, DNS, certs, and registry
outport system start        # reinstall from scratch
```

Then re-register each project with `outport up`.

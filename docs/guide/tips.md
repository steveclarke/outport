# Tips & Troubleshooting

## protocol: http vs https

Set `protocol: http` even though your `.test` URLs show `https://`. The `protocol` field describes what your app speaks — your Rails server, Nuxt dev server, and Mailpit all listen on plain HTTP. The outport daemon terminates TLS at the proxy and forwards to your app over HTTP.

The URLs automatically show `https://` when the local CA is installed (after `outport system start`). You don't need to change `protocol` to make that happen.

## Daemon not running

If `outport up` shows this hint:

```
Hint: The outport daemon is not running.
Run 'outport system start' to enable .test domains.
```

Port allocation and `.env` writing still work without the daemon — you just won't have `.test` hostnames or HTTPS. Run `outport system start` to enable them.

## After upgrading outport

After `brew upgrade outport`, bounce the daemon to pick up the new binary:

```bash
outport system restart
```

This re-writes the LaunchAgent plist with the new binary path and restarts the daemon.

## Ports changed unexpectedly

Outport allocates deterministic ports — the same project, instance, and service name always produce the same port. If your ports changed, one of these happened:

- **You re-registered from a different directory** — the instance name may have changed
- **You used `--force`** — this re-allocates all ports from scratch
- **Another project claimed your preferred port** — preferred ports are first-come-first-served across all registered projects

Run `outport ports` to see your current allocations and `outport system status` to see all registered projects.

## Stale registry entries

If `outport system status` shows stale entries (projects whose directories no longer exist):

```bash
outport system gc
```

This removes entries where the project directory or `.outport.yml` is missing.

## Multiple worktrees of the same project

Each worktree gets its own instance with unique ports and suffixed hostnames:

```bash
cd ~/src/myapp           # main checkout
outport up               # myapp [main] → myapp.test

cd ~/src/myapp-feature   # worktree
outport up               # myapp [xbjf] → myapp-xbjf.test
```

To give a worktree a readable name:

```bash
outport rename xbjf feature
# Now: myapp-feature.test
```

To make a worktree the primary checkout:

```bash
outport promote
# This worktree becomes [main], the old main gets a generated code
```

## Docker Compose conflicts with worktrees

If `docker compose up` from one worktree replaces another's containers, add a `COMPOSE_PROJECT_NAME` derived value:

```yaml
derived:
  COMPOSE_PROJECT_NAME:
    value: "myapp${instance:+-${instance}}"
    env_file: .env
```

This gives each instance a unique Docker Compose project name.

## Port 80 or 443 already in use

If `outport system start` fails with "port 80 is already in use", another server (nginx, Apache, another dev tool) is using that port. Stop it first, then retry.

On macOS, find what's using the port:

```bash
sudo lsof -iTCP:80 -sTCP:LISTEN
```

## Clean reinstall

To start completely fresh:

```bash
outport system uninstall    # removes daemon, DNS, certs, and registry
outport system start        # reinstall from scratch
```

Then re-register each project with `outport up`.

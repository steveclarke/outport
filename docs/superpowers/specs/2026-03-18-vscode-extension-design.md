# Outport VS Code Extension

## Problem

After running `bin/dev` or `bin/services`, developers have to switch to a terminal and run `outport ports` to find their allocated URLs and ports. There's a gap between where you configure your dev environment and where you use it. New users also find `outport.yml` authoring intimidating without editor guidance.

## Solution

A VS Code extension that surfaces Outport's runtime state directly in the editor and provides intelligent config authoring support. The extension never reimplements Outport logic — it shells out to the CLI and parses `--json` output.

## Audience

Power users who want convenience (clickable URLs, health at a glance) and newcomers who benefit from config validation and autocomplete in their editor. The extension should work silently out of the box — no popups, no prompts, no auto-running commands.

## Design

### Sidebar Panel

The extension registers an "Outport" view in the Explorer sidebar. It reads from `outport ports --json --check --computed` to populate a tree view.

**Tree structure:**

```
OUTPORT
├── myapp [main]
│   ├── ● web          PORT=24920    https://myapp.test
│   ├── ○ postgres     DB_PORT=5432
│   └── ● redis        REDIS_PORT=29454
│   └── Computed
│       ├── CORS_ORIGINS = https://myapp.test
│       └── API_URL = http://localhost:24920/api
└── Actions
    ├── ▶ Run Up
    ├── ▶ Run Up --force
    └── ▶ Run Down
```

- **Health dots**: green = port is listening, red = not listening (mirrors `--check` output)
- **Clickable URLs**: services with `protocol: http/https` show their URL; clicking opens in default browser
- **Copy actions**: right-click any port, URL, or env var to copy to clipboard
- **Instance label**: shows `[main]` or `[bxcf]` so you always know which checkout you're in
- **Computed values**: collapsible section showing resolved template values
- **Actions**: buttons to run `outport up`, `outport up --force`, `outport down` with output shown in an Outport Output channel
- **Auto-refresh**: watches the registry file (`~/.local/share/outport/registry.json`) for changes using VS Code's `FileSystemWatcher`. No polling.

### Status Bar

Minimal — one item, left side:

- **Active project**: `$(globe) myapp [main]`
- **No config found**: item is hidden entirely
- **Config found but not registered**: `$(globe) myapp (run outport up)`
- **Click behavior**: focuses the Outport sidebar panel

No notifications, no popups, no second status bar item.

### Workspace Awareness

- **Single-root workspace**: finds `outport.yml` starting from the workspace folder root and walking up to parent directories (same as the CLI's `FindDir()`). This means a config in a parent of the workspace root will be found. One project, one sidebar tree.
- **Multi-root workspace**: each workspace folder is checked independently. The sidebar shows a section per project. If two folders are different instances of the same project (e.g., main checkout and a worktree), both appear in the sidebar — this is intentional and useful.
- **Worktree windows**: each VS Code window has its own workspace root, so each gets its own instance resolved automatically. The status bar indicator shows which instance you're in.
- **Nested configs**: follows the same semantics as the CLI — finds the nearest `outport.yml` walking up from each workspace folder root. See [#50](https://github.com/steveclarke/outport/issues/50) for future discussion on shadowing warnings.

### Config Authoring (`outport.yml` Intelligence)

Three layers, shipped incrementally:

**Layer 1: JSON Schema (MVP)**

A JSON Schema for `outport.yml` consumed by the Red Hat YAML extension (`redhat.vscode-yaml`). Provides:

- Autocomplete for top-level keys (`name`, `services`, `computed`)
- Autocomplete for service fields (`env_var`, `preferred_port`, `protocol`, `hostname`, `env_file`)
- Type validation (`preferred_port` must be a number, `protocol` must be `http` or `https`)
- Required field warnings (missing `env_var`, missing `name`)
- Hover docs explaining each field

The schema is contributed automatically by the extension — no user configuration needed.

**Layer 2: Custom Diagnostics (v0.2.0)**

Real-time validation mirroring `config.validate()`:

- Hostname must contain project name
- Duplicate `env_var` per file
- Computed value references unknown service
- Computed value name collides with a service `env_var`
- Missing `value` when per-file overrides don't cover all entries

These run on file save/change via VS Code's `DiagnosticCollection`.

**Layer 3: Template Intelligence (v0.3.0+)**

- Autocomplete inside `${}` — type `${` and see service names, then `.port`, `.hostname`, `.url`, `.url:direct`
- Autocomplete for standalone vars: `${instance}`, `${instance:-default}`, `${instance:+replacement}`
- Hover over `${rails.url}` shows resolved value (reads from registry if available)
- Go-to-definition on `${rails.port}` jumps to the `rails:` service definition

## Error Handling

The extension must handle CLI failures gracefully since it depends entirely on shelling out to `outport`.

**`outport` not installed or not on `$PATH`:**
- Sidebar shows a single tree item: "Outport CLI not found" with a link to installation docs
- Status bar is hidden
- Config authoring (JSON Schema) still works — it has no CLI dependency

**Config found but `outport ports` fails (user hasn't run `outport up`):**
- Sidebar shows the project name with "(run outport up)" and the action buttons
- Status bar shows `$(globe) myapp (run outport up)`

**CLI returns non-zero exit code (invalid config, corrupt registry, etc.):**
- Sidebar shows the project name with the error message from stderr
- The error is also logged to the Outport Output channel

**No `outport.yml` found in workspace:**
- Sidebar shows "No outport.yml found" with a hint to run `outport init`
- Status bar is hidden

## Technology & Project Structure

**Language**: TypeScript.

**Repo**: `steveclarke/outport-vscode` — standalone repo, separate from the Go CLI.

**Binary path resolution**: The extension looks for `outport` on `$PATH` by default. A `outport.binaryPath` setting allows overriding this for non-standard installations. The extension inherits `$PATH` from the user's shell profile on activation.

**Architecture:**

```
outport-vscode/
├── src/
│   ├── extension.ts          # activate/deactivate, register everything
│   ├── sidebar/
│   │   ├── provider.ts       # TreeDataProvider for the sidebar
│   │   └── items.ts          # TreeItem types (project, service, computed, action)
│   ├── statusbar.ts          # Status bar indicator
│   ├── config/
│   │   ├── schema.json       # JSON Schema for outport.yml
│   │   └── diagnostics.ts    # Layer 2 custom validation diagnostics
│   ├── cli.ts                # Wrapper: shells out to `outport` CLI, parses JSON
│   └── watcher.ts            # FileSystemWatcher on registry.json
├── package.json              # Extension manifest, contributes views/commands
└── tsconfig.json
```

**CLI interaction model**: the extension shells out to `outport ports --json`, `outport system status --json`, `outport up --json`, etc. and parses the structured output. This means:

- The extension always agrees with the CLI
- New Outport features automatically surface in JSON output
- No version coupling beyond the JSON contract

**Refresh strategy:**

- Watch `~/.local/share/outport/registry.json` — any `outport up` or `outport down` triggers sidebar refresh. This is the sole refresh trigger; action buttons (Run Up, Run Down) do not separately refresh on completion since the registry write will trigger the watcher.
- Watch `outport.yml` in the workspace — changes trigger re-validation diagnostics
- No polling, no timers

**File watcher note**: The registry file is outside the workspace, so the watcher uses an absolute `GlobPattern`. The registry is written atomically via temp file + rename — the implementer should verify that `FileSystemWatcher` picks up rename events on macOS (FSEvents). If not, a fallback of refreshing on action button completion may be needed.

**JSON Schema limitations**: The schema covers structure and types but cannot validate template strings like `${rails.url:direct}`. Template validity is the job of Layer 2 custom diagnostics. The schema also cannot express that `env_file` on computed values accepts three shapes (string, string array, array of `{file, value}` objects) with full fidelity — `oneOf`/`anyOf` gets close but may produce confusing error messages for invalid input. This is acceptable; Layer 2 diagnostics will provide better errors.

## Milestones

### MVP (v0.1.0)

- Sidebar panel with service tree (name, env_var, port, URL, health dots)
- Clickable URLs to open in browser
- Copy-to-clipboard on right-click (port, URL, env_var)
- Computed values section (collapsible)
- Instance label (`[main]` / `[bxcf]`)
- Action buttons: Run Up, Run Up --force, Run Down (output to Output channel)
- Status bar indicator: `$(globe) myapp [main]` / hidden when no config
- JSON Schema for `outport.yml` (Layer 1 config authoring)
- Registry file watcher for auto-refresh
- Multi-root workspace support

### Fast Follow (v0.2.0)

- Layer 2 diagnostics (custom validation mirroring `config.validate()`)
- `outport doctor` integration — show warnings in sidebar when checks fail
- `outport share` action with tunnel URLs displayed in sidebar

### Stretch (v0.3.0+)

- Layer 3 template intelligence (`${}` autocomplete, hover resolution, go-to-definition)
- `outport init` scaffold from command palette with guided prompts
- Webview panel for a richer UI (if tree view feels too limiting)

## Out of Scope

- Auto-running `outport up` on workspace open
- Notifications or popups of any kind
- Reimplementing any Outport logic (always shell out to CLI)
- Managing `outport system` commands (start/stop/restart) — those require sudo and are one-time setup

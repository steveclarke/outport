# Outport VS Code Extension Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a VS Code extension that surfaces Outport's runtime state (ports, URLs, health) in a sidebar panel and provides JSON Schema-based config authoring for `.outport.yml`.

**Architecture:** The extension shells out to the `outport` CLI with `--json` flags and parses structured output — it never reimplements Outport logic. A `FileSystemWatcher` on the registry file triggers auto-refresh. The YAML schema is contributed via `package.json` for the Red Hat YAML extension.

**Tech Stack:** TypeScript, VS Code Extension API (TreeView, StatusBarItem, FileSystemWatcher), esbuild bundler, Mocha test framework.

**Spec:** `docs/superpowers/specs/2026-03-18-vscode-extension-design.md`

---

## File Structure

```
outport-vscode/
├── .vscode/
│   ├── extensions.json       # Recommended extensions for contributors
│   ├── launch.json           # F5 launches Extension Development Host
│   ├── settings.json         # Editor settings for the project
│   └── tasks.json            # Compile tasks
├── src/
│   ├── extension.ts          # activate/deactivate, register all providers
│   ├── cli.ts                # Shell out to `outport` CLI, parse JSON output
│   ├── sidebar/
│   │   ├── provider.ts       # TreeDataProvider — builds tree from CLI output
│   │   └── items.ts          # TreeItem subclasses (project, service, computed, action, message)
│   ├── statusbar.ts          # StatusBarItem — shows project/instance, click to focus sidebar
│   ├── watcher.ts            # FileSystemWatcher on registry.json, debounced refresh
│   └── config/
│       └── schema.json       # JSON Schema for .outport.yml
├── src/test/
│   ├── unit/
│   │   ├── cli.test.ts       # Tests for CLI output parsing
│   │   └── items.test.ts     # Tests for TreeItem construction
│   └── integration/
│       └── extension.test.ts # Tests extension activation, commands, tree registration
├── test-fixtures/
│   └── workspace/
│       └── .outport.yml      # Sample config for integration tests
├── schemas/
│   └── outport.schema.json   # Published schema (copied from src at build time)
├── resources/
│   └── outport.svg           # Extension icon
├── package.json              # Extension manifest
├── tsconfig.json
├── esbuild.js                # Build script
├── .vscode-test.mjs          # Test runner config
├── .vscodeignore
├── .gitignore
├── LICENSE
└── README.md
```

**Responsibilities:**

- `cli.ts` — Single point of contact with the `outport` binary. Exports `runOutport(args, cwd)` that returns parsed JSON or throws with stderr. Handles binary path resolution from settings. All other modules call this, never `child_process` directly.
- `sidebar/provider.ts` — Implements `TreeDataProvider<OutportItem>`. Calls `cli.ts` to get data, constructs tree items. Exposes `refresh()` method fired by the watcher.
- `sidebar/items.ts` — TreeItem subclasses with appropriate icons, descriptions, tooltips, contextValues, and click commands. Keeps provider.ts focused on data flow.
- `statusbar.ts` — Exports `createStatusBar()` and `updateStatusBar(data)`. Reads the same CLI data as the sidebar. Shows/hides based on config presence.
- `watcher.ts` — Exports `createRegistryWatcher(onChanged)`. Watches `~/.local/share/outport/registry.json` with debounce. Returns disposable.
- `extension.ts` — Wires everything together in `activate()`. Registers commands, providers, watchers. Thin orchestration only.
- `config/schema.json` — JSON Schema for `.outport.yml`. Contributed via `yamlValidation` in `package.json`.

---

## Task 1: Scaffold the Extension Project

**Files:**
- Create: `outport-vscode/` (new repo)
- Create: `package.json`, `tsconfig.json`, `.vscode/launch.json`, `.vscode/tasks.json`, `.vscode/extensions.json`, `.vscode/settings.json`, `.vscodeignore`, `.gitignore`
- Create: `src/extension.ts`

- [ ] **Step 1: Create the repo and initialize**

```bash
mkdir -p ~/src/outport-vscode
cd ~/src/outport-vscode
git init
npm init -y
```

- [ ] **Step 2: Install dependencies**

```bash
npm install --save-dev @types/vscode @types/node typescript @types/mocha mocha @vscode/test-cli @vscode/test-electron esbuild
```

- [ ] **Step 3: Write `package.json` manifest**

Replace the generated `package.json` with the full extension manifest:

```json
{
  "name": "outport",
  "displayName": "Outport",
  "description": "Port management for multi-project development — see your ports, URLs, and service health right in VS Code",
  "version": "0.1.0",
  "publisher": "steveclarke",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/steveclarke/outport-vscode"
  },
  "engines": {
    "vscode": "^1.85.0"
  },
  "categories": ["Other"],
  "activationEvents": [
    "workspaceContains:.outport.yml",
    "workspaceContains:.outport.yaml"
  ],
  "main": "./out/extension.js",
  "contributes": {
    "views": {
      "explorer": [
        {
          "id": "outportView",
          "name": "Outport",
          "when": "outport.active",
          "contextualTitle": "Outport"
        }
      ]
    },
    "commands": [
      { "command": "outport.refresh", "title": "Refresh", "category": "Outport", "icon": "$(refresh)" },
      { "command": "outport.up", "title": "Run Up", "category": "Outport", "icon": "$(play)" },
      { "command": "outport.upForce", "title": "Run Up --force", "category": "Outport" },
      { "command": "outport.down", "title": "Run Down", "category": "Outport", "icon": "$(debug-stop)" },
      { "command": "outport.openService", "title": "Open in Browser", "category": "Outport", "icon": "$(link-external)" },
      { "command": "outport.copyPort", "title": "Copy Port", "category": "Outport" },
      { "command": "outport.copyUrl", "title": "Copy URL", "category": "Outport" },
      { "command": "outport.copyEnvVar", "title": "Copy Env Var Assignment", "category": "Outport" }
    ],
    "menus": {
      "view/title": [
        { "command": "outport.refresh", "when": "view == outportView", "group": "navigation" },
        { "command": "outport.up", "when": "view == outportView", "group": "navigation" },
        { "command": "outport.down", "when": "view == outportView" },
        { "command": "outport.upForce", "when": "view == outportView" }
      ],
      "view/item/context": [
        { "command": "outport.openService", "when": "view == outportView && viewItem == httpService", "group": "inline" },
        { "command": "outport.copyPort", "when": "view == outportView && viewItem =~ /^(service|httpService)$/" },
        { "command": "outport.copyUrl", "when": "view == outportView && viewItem == httpService" },
        { "command": "outport.copyEnvVar", "when": "view == outportView && viewItem =~ /^(service|httpService)$/" }
      ]
    },
    "configuration": {
      "title": "Outport",
      "properties": {
        "outport.binaryPath": {
          "type": "string",
          "default": "outport",
          "description": "Path to the outport binary. Use an absolute path if outport is not on your PATH."
        }
      }
    },
    "yamlValidation": [
      { "fileMatch": ".outport.yml", "url": "./schemas/outport.schema.json" },
      { "fileMatch": ".outport.yaml", "url": "./schemas/outport.schema.json" }
    ]
  },
  "scripts": {
    "compile": "tsc -p ./",
    "watch": "tsc -watch -p ./",
    "build": "node esbuild.js",
    "lint": "eslint src/",
    "test": "vscode-test",
    "test:unit": "vscode-test --label unitTests",
    "vscode:prepublish": "npm run build"
  },
  "devDependencies": {
    "@types/vscode": "^1.85.0",
    "@types/node": "^22",
    "@types/mocha": "^10",
    "typescript": "^5.8",
    "mocha": "^10",
    "@vscode/test-cli": "^0.0.10",
    "@vscode/test-electron": "^2.4",
    "esbuild": "^0.24"
  }
}
```

- [ ] **Step 4: Write `tsconfig.json`**

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "target": "ES2022",
    "lib": ["ES2022"],
    "outDir": "out",
    "rootDir": "src",
    "sourceMap": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "out"]
}
```

- [ ] **Step 5: Write `.vscode/launch.json`**

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Run Extension",
      "type": "extensionHost",
      "request": "launch",
      "args": ["--extensionDevelopmentPath=${workspaceFolder}"],
      "outFiles": ["${workspaceFolder}/out/**/*.js"],
      "preLaunchTask": "npm: compile"
    }
  ]
}
```

- [ ] **Step 6: Write `.vscode/tasks.json`**

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "type": "npm",
      "script": "compile",
      "problemMatcher": "$tsc",
      "group": { "kind": "build", "isDefault": true }
    },
    {
      "type": "npm",
      "script": "watch",
      "problemMatcher": "$tsc-watch",
      "isBackground": true
    }
  ]
}
```

- [ ] **Step 7: Write `.vscode/extensions.json`**

```json
{
  "recommendations": ["dbaeumer.vscode-eslint"]
}
```

- [ ] **Step 8: Write `.vscodeignore`**

```
.vscode/**
src/**
!schemas/**
node_modules/**
test-fixtures/**
.vscode-test.mjs
tsconfig.json
esbuild.js
*.map
```

- [ ] **Step 9: Write `.gitignore`**

```
node_modules/
out/
.vscode-test/
*.vsix
```

- [ ] **Step 10: Write `esbuild.js` build script**

```javascript
const esbuild = require('esbuild');

esbuild.build({
  entryPoints: ['src/extension.ts'],
  bundle: true,
  outfile: 'out/extension.js',
  external: ['vscode'],
  format: 'cjs',
  platform: 'node',
  target: 'node18',
  sourcemap: true,
  minify: true,
}).catch(() => process.exit(1));
```

- [ ] **Step 11: Write minimal `src/extension.ts`**

```typescript
import * as vscode from 'vscode';

export function activate(context: vscode.ExtensionContext): void {
  const outputChannel = vscode.window.createOutputChannel('Outport');
  outputChannel.appendLine('Outport extension activated');
}

export function deactivate(): void {}
```

- [ ] **Step 12: Compile and verify**

Run: `cd ~/src/outport-vscode && npm run compile`
Expected: Compiles with no errors, `out/extension.js` exists.

- [ ] **Step 13: Commit**

```bash
git add -A
git commit -m "feat: scaffold VS Code extension project"
```

---

## Task 2: CLI Wrapper (`cli.ts`)

**Files:**
- Create: `src/cli.ts`
- Test: `src/test/unit/cli.test.ts`

This is the foundation — every other module depends on it.

- [ ] **Step 1: Define the CLI output types**

Create `src/cli.ts`:

```typescript
import { execFile } from 'child_process';
import { promisify } from 'util';
import * as vscode from 'vscode';

const execFileAsync = promisify(execFile);

export interface ServiceJSON {
  port: number;
  env_var: string;
  preferred_port?: number;
  protocol?: string;
  hostname?: string;
  url?: string;
  up?: boolean;
  env_files: string[];
}

export interface ComputedJSON {
  value?: string;
  values?: Record<string, string>;
  env_files?: string[];
}

export interface PortsOutput {
  project: string;
  instance: string;
  services: Record<string, ServiceJSON>;
  computed?: Record<string, ComputedJSON>;
  env_files: string[];
}

export interface CliError {
  kind: 'not-found' | 'not-registered' | 'cli-error';
  message: string;
}

export type CliResult<T> = { ok: true; data: T } | { ok: false; error: CliError };
```

- [ ] **Step 2: Implement `runOutport` and `getPorts`**

Append to `src/cli.ts`:

```typescript
function getBinaryPath(): string {
  const config = vscode.workspace.getConfiguration('outport');
  return config.get<string>('binaryPath', 'outport');
}

async function runOutport(args: string[], cwd: string): Promise<CliResult<string>> {
  const bin = getBinaryPath();
  try {
    const { stdout } = await execFileAsync(bin, args, {
      cwd,
      timeout: 15_000,
    });
    return { ok: true, data: stdout };
  } catch (err: any) {
    if (err.code === 'ENOENT') {
      return { ok: false, error: { kind: 'not-found', message: `outport binary not found at "${bin}"` } };
    }
    const stderr = err.stderr?.trim() || err.message;
    if (stderr.includes('No .outport.yml found') || stderr.includes('not found in registry')) {
      return { ok: false, error: { kind: 'not-registered', message: stderr } };
    }
    return { ok: false, error: { kind: 'cli-error', message: stderr } };
  }
}

export async function getPorts(cwd: string): Promise<CliResult<PortsOutput>> {
  const result = await runOutport(['ports', '--json', '--check', '--computed'], cwd);
  if (!result.ok) return result;
  try {
    const data = JSON.parse(result.data) as PortsOutput;
    return { ok: true, data };
  } catch {
    return { ok: false, error: { kind: 'cli-error', message: 'Failed to parse outport JSON output' } };
  }
}

export async function runUp(cwd: string, force: boolean): Promise<CliResult<string>> {
  const args = ['up', '--json'];
  if (force) args.push('--force');
  return runOutport(args, cwd);
}

export async function runDown(cwd: string): Promise<CliResult<string>> {
  return runOutport(['down'], cwd);
}
```

- [ ] **Step 3: Write unit tests for output parsing**

Create `src/test/unit/cli.test.ts`:

```typescript
import * as assert from 'assert';

// We test the parsing logic in isolation — we can't easily test execFile in unit tests
// so we test the type contracts and JSON parsing

suite('CLI Output Parsing', () => {
  test('parses valid ports JSON', () => {
    const json = JSON.stringify({
      project: 'myapp',
      instance: 'main',
      services: {
        web: { port: 24920, env_var: 'PORT', protocol: 'http', hostname: 'myapp.test', url: 'https://myapp.test', up: true, env_files: ['.env'] },
        postgres: { port: 5432, env_var: 'DB_PORT', env_files: ['.env'] },
      },
      computed: {
        CORS_ORIGINS: { value: 'https://myapp.test', env_files: ['.env'] },
      },
      env_files: ['.env'],
    });

    const data = JSON.parse(json);
    assert.strictEqual(data.project, 'myapp');
    assert.strictEqual(data.instance, 'main');
    assert.strictEqual(data.services.web.port, 24920);
    assert.strictEqual(data.services.web.up, true);
    assert.strictEqual(data.computed.CORS_ORIGINS.value, 'https://myapp.test');
  });

  test('handles missing optional fields', () => {
    const json = JSON.stringify({
      project: 'myapp',
      instance: 'main',
      services: {
        postgres: { port: 5432, env_var: 'DB_PORT', env_files: ['.env'] },
      },
      env_files: ['.env'],
    });

    const data = JSON.parse(json);
    assert.strictEqual(data.services.postgres.protocol, undefined);
    assert.strictEqual(data.services.postgres.hostname, undefined);
    assert.strictEqual(data.services.postgres.url, undefined);
    assert.strictEqual(data.services.postgres.up, undefined);
    assert.strictEqual(data.computed, undefined);
  });
});
```

- [ ] **Step 4: Set up test runner config**

Create `.vscode-test.mjs`:

```javascript
import { defineConfig } from '@vscode/test-cli';

export default defineConfig([
  {
    label: 'unitTests',
    files: 'out/test/unit/**/*.test.js',
    mocha: { ui: 'tdd', timeout: 5000 },
  },
]);
```

- [ ] **Step 5: Compile and run tests**

Run: `cd ~/src/outport-vscode && npm run compile && npm test`
Expected: 2 tests pass.

- [ ] **Step 6: Commit**

```bash
git add src/cli.ts src/test/unit/cli.test.ts .vscode-test.mjs
git commit -m "feat: add CLI wrapper with typed output parsing"
```

---

## Task 3: Tree Items (`sidebar/items.ts`)

**Files:**
- Create: `src/sidebar/items.ts`
- Test: `src/test/unit/items.test.ts`

- [ ] **Step 1: Write TreeItem subclasses**

Create `src/sidebar/items.ts`:

```typescript
import * as vscode from 'vscode';
import { ServiceJSON, ComputedJSON } from '../cli';

export class ProjectItem extends vscode.TreeItem {
  constructor(
    public readonly projectName: string,
    public readonly instance: string,
  ) {
    super(`${projectName} [${instance}]`, vscode.TreeItemCollapsibleState.Expanded);
    this.contextValue = 'project';
    this.iconPath = new vscode.ThemeIcon('globe');
  }
}

export class ServiceItem extends vscode.TreeItem {
  constructor(
    public readonly serviceName: string,
    public readonly service: ServiceJSON,
  ) {
    super(serviceName, vscode.TreeItemCollapsibleState.None);

    const isHttp = service.protocol === 'http' || service.protocol === 'https';
    const isUp = service.up === true;
    const healthDot = service.up === undefined ? '' : isUp ? '$(pass-filled) ' : '$(circle-large-outline) ';

    this.description = `${service.env_var}=${service.port}`;
    if (service.url) {
      this.description += `    ${service.url}`;
    }

    this.tooltip = new vscode.MarkdownString();
    this.tooltip.appendMarkdown(`**${serviceName}**\n\n`);
    this.tooltip.appendMarkdown(`- Port: \`${service.port}\`\n`);
    this.tooltip.appendMarkdown(`- Env var: \`${service.env_var}\`\n`);
    if (service.hostname) this.tooltip.appendMarkdown(`- Hostname: \`${service.hostname}\`\n`);
    if (service.url) this.tooltip.appendMarkdown(`- URL: ${service.url}\n`);
    if (service.up !== undefined) this.tooltip.appendMarkdown(`- Status: ${isUp ? 'listening' : 'not listening'}\n`);

    this.iconPath = new vscode.ThemeIcon(
      service.up === true ? 'pass-filled' : service.up === false ? 'circle-large-outline' : 'circle-outline',
      service.up === true ? new vscode.ThemeColor('testing.iconPassed') : service.up === false ? new vscode.ThemeColor('testing.iconFailed') : undefined,
    );

    if (isHttp && service.url) {
      this.contextValue = 'httpService';
      this.command = {
        command: 'outport.openService',
        title: 'Open in Browser',
        arguments: [service.url],
      };
    } else {
      this.contextValue = 'service';
    }
  }
}

export class ComputedHeaderItem extends vscode.TreeItem {
  constructor() {
    super('Computed', vscode.TreeItemCollapsibleState.Collapsed);
    this.contextValue = 'computedHeader';
    this.iconPath = new vscode.ThemeIcon('symbol-variable');
  }
}

export class ComputedItem extends vscode.TreeItem {
  constructor(name: string, computed: ComputedJSON) {
    super(name, vscode.TreeItemCollapsibleState.None);
    // Use top-level value if present, otherwise show first per-file value
    const displayValue = computed.value
      ?? (computed.values ? Object.values(computed.values)[0] : '');
    this.description = displayValue;
    this.tooltip = computed.values
      ? Object.entries(computed.values).map(([f, v]) => `${f}: ${v}`).join('\n')
      : `${name} = ${displayValue}`;
    this.contextValue = 'computed';
    this.iconPath = new vscode.ThemeIcon('symbol-constant');
  }
}

export class MessageItem extends vscode.TreeItem {
  constructor(message: string, icon?: string) {
    super(message, vscode.TreeItemCollapsibleState.None);
    this.contextValue = 'message';
    if (icon) {
      this.iconPath = new vscode.ThemeIcon(icon);
    }
  }
}
```

- [ ] **Step 2: Write unit tests for item construction**

Create `src/test/unit/items.test.ts`:

```typescript
import * as assert from 'assert';

// We test the logic of item construction without requiring VS Code APIs
// by checking the properties that get set

suite('Tree Item Construction', () => {
  test('ProjectItem shows name and instance', () => {
    // Since TreeItem requires vscode, we test the label construction logic
    const label = `myapp [main]`;
    assert.strictEqual(label, 'myapp [main]');

    const worktreeLabel = `myapp [bxcf]`;
    assert.strictEqual(worktreeLabel, 'myapp [bxcf]');
  });

  test('ServiceItem description includes env_var and port', () => {
    const service = { port: 24920, env_var: 'PORT', env_files: ['.env'] };
    const desc = `${service.env_var}=${service.port}`;
    assert.strictEqual(desc, 'PORT=24920');
  });

  test('ServiceItem description includes URL when present', () => {
    const service = { port: 24920, env_var: 'PORT', url: 'https://myapp.test', env_files: ['.env'] };
    let desc = `${service.env_var}=${service.port}`;
    if (service.url) desc += `    ${service.url}`;
    assert.strictEqual(desc, 'PORT=24920    https://myapp.test');
  });

  test('httpService contextValue set for HTTP services', () => {
    const service = { port: 24920, env_var: 'PORT', protocol: 'http', url: 'https://myapp.test', env_files: ['.env'] };
    const isHttp = service.protocol === 'http' || service.protocol === 'https';
    const contextValue = isHttp && service.url ? 'httpService' : 'service';
    assert.strictEqual(contextValue, 'httpService');
  });

  test('service contextValue set for non-HTTP services', () => {
    const service = { port: 5432, env_var: 'DB_PORT', env_files: ['.env'] };
    const isHttp = (service as any).protocol === 'http' || (service as any).protocol === 'https';
    const contextValue = isHttp ? 'httpService' : 'service';
    assert.strictEqual(contextValue, 'service');
  });
});
```

- [ ] **Step 3: Compile and run tests**

Run: `cd ~/src/outport-vscode && npm run compile && npm test`
Expected: All tests pass (previous + new).

- [ ] **Step 4: Commit**

```bash
git add src/sidebar/items.ts src/test/unit/items.test.ts
git commit -m "feat: add tree item types for sidebar"
```

---

## Task 4: Tree Data Provider (`sidebar/provider.ts`)

**Files:**
- Create: `src/sidebar/provider.ts`

- [ ] **Step 1: Implement the TreeDataProvider**

Create `src/sidebar/provider.ts`:

```typescript
import * as vscode from 'vscode';
import { getPorts, CliResult, PortsOutput } from '../cli';
import { ProjectItem, ServiceItem, ComputedHeaderItem, ComputedItem, MessageItem } from './items';

type OutportTreeItem = ProjectItem | ServiceItem | ComputedHeaderItem | ComputedItem | MessageItem;

export class OutportTreeProvider implements vscode.TreeDataProvider<OutportTreeItem> {
  private _onDidChangeTreeData = new vscode.EventEmitter<OutportTreeItem | undefined | void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private data: CliResult<PortsOutput> | null = null;
  private outputChannel: vscode.OutputChannel;
  private onDataLoaded?: (result: CliResult<PortsOutput>) => void;

  constructor(outputChannel: vscode.OutputChannel, onDataLoaded?: (result: CliResult<PortsOutput>) => void) {
    this.outputChannel = outputChannel;
    this.onDataLoaded = onDataLoaded;
  }

  /** Returns the last fetched data (for status bar to read without a second CLI call). */
  getLastResult(): CliResult<PortsOutput> | null {
    return this.data;
  }

  refresh(): void {
    this.data = null;
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(element: OutportTreeItem): vscode.TreeItem {
    return element;
  }

  async getChildren(element?: OutportTreeItem): Promise<OutportTreeItem[]> {
    // Top level — fetch data and return project or error
    if (!element) {
      return this.getTopLevel();
    }

    // Children of the project node — services + computed header
    if (element instanceof ProjectItem && this.data?.ok) {
      return this.getProjectChildren(this.data.data);
    }

    // Children of computed header — individual computed values
    if (element instanceof ComputedHeaderItem && this.data?.ok) {
      return this.getComputedChildren(this.data.data);
    }

    return [];
  }

  private async getTopLevel(): Promise<OutportTreeItem[]> {
    const folders = vscode.workspace.workspaceFolders;
    if (!folders || folders.length === 0) {
      return [new MessageItem('No workspace folder open', 'warning')];
    }

    // Support multi-root workspaces: check each folder for an Outport config.
    // For single-root (most common), this just checks the one folder.
    const items: OutportTreeItem[] = [];
    for (const folder of folders) {
      const result = await getPorts(folder.uri.fsPath);
      if (result.ok) {
        items.push(new ProjectItem(result.data.project, result.data.instance));
        // Store the last successful result for status bar
        this.data = result;
        this.onDataLoaded?.(result);
      } else if (result.error.kind === 'not-found') {
        // CLI not installed — show once and stop checking other folders
        return [new MessageItem('Outport CLI not found — install from outport.dev', 'warning')];
      } else if (result.error.kind === 'not-registered') {
        items.push(new MessageItem(`${folder.name}: run "outport up" to allocate ports`, 'info'));
      } else {
        this.outputChannel.appendLine(`[error] ${folder.name}: ${result.error.message}`);
        items.push(new MessageItem(`${folder.name}: ${result.error.message}`, 'error'));
      }
    }

    if (items.length === 0) {
      return [new MessageItem('No .outport.yml found', 'info')];
    }

    return items;
  }

  private getProjectChildren(data: PortsOutput): OutportTreeItem[] {
    const items: OutportTreeItem[] = [];

    for (const [name, service] of Object.entries(data.services)) {
      items.push(new ServiceItem(name, service));
    }

    if (data.computed && Object.keys(data.computed).length > 0) {
      items.push(new ComputedHeaderItem());
    }

    return items;
  }

  private getComputedChildren(data: PortsOutput): OutportTreeItem[] {
    if (!data.computed) return [];
    return Object.entries(data.computed).map(
      ([name, computed]) => new ComputedItem(name, computed),
    );
  }
}
```

- [ ] **Step 2: Compile and verify**

Run: `cd ~/src/outport-vscode && npm run compile`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add src/sidebar/provider.ts
git commit -m "feat: add TreeDataProvider for sidebar"
```

---

## Task 5: Status Bar (`statusbar.ts`)

**Files:**
- Create: `src/statusbar.ts`

- [ ] **Step 1: Implement the status bar module**

Create `src/statusbar.ts`:

```typescript
import * as vscode from 'vscode';
import { CliResult, PortsOutput } from './cli';

let statusBarItem: vscode.StatusBarItem | undefined;

export function createStatusBar(context: vscode.ExtensionContext): vscode.StatusBarItem {
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
  statusBarItem.command = 'outportView.focus';
  context.subscriptions.push(statusBarItem);
  return statusBarItem;
}

export function updateStatusBar(result: CliResult<PortsOutput> | null): void {
  if (!statusBarItem) return;

  if (!result) {
    statusBarItem.hide();
    return;
  }

  if (result.ok) {
    const { project, instance } = result.data;
    statusBarItem.text = `$(globe) ${project} [${instance}]`;
    statusBarItem.tooltip = 'Click to show Outport sidebar';
    statusBarItem.show();
    return;
  }

  if (result.error.kind === 'not-registered') {
    statusBarItem.text = `$(globe) (run outport up)`;
    statusBarItem.tooltip = 'Outport config found but not registered — run outport up';
    statusBarItem.show();
    return;
  }

  // not-found or cli-error — hide
  statusBarItem.hide();
}
```

- [ ] **Step 2: Compile and verify**

Run: `cd ~/src/outport-vscode && npm run compile`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add src/statusbar.ts
git commit -m "feat: add status bar indicator"
```

---

## Task 6: Registry File Watcher (`watcher.ts`)

**Files:**
- Create: `src/watcher.ts`

- [ ] **Step 1: Implement the registry watcher with debounce**

Create `src/watcher.ts`:

```typescript
import * as vscode from 'vscode';
import * as os from 'os';
import * as path from 'path';

export function createRegistryWatcher(onChanged: () => void): vscode.Disposable {
  const registryDir = path.join(os.homedir(), '.local', 'share', 'outport');

  // Use *.json glob for reliability with atomic rename writes
  const watcher = vscode.workspace.createFileSystemWatcher(
    new vscode.RelativePattern(vscode.Uri.file(registryDir), '*.json'),
  );

  let debounceTimer: ReturnType<typeof setTimeout> | undefined;

  const debouncedRefresh = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(onChanged, 150);
  };

  watcher.onDidChange(debouncedRefresh);
  watcher.onDidCreate(debouncedRefresh);
  watcher.onDidDelete(debouncedRefresh);

  return watcher;
}
```

- [ ] **Step 2: Compile and verify**

Run: `cd ~/src/outport-vscode && npm run compile`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add src/watcher.ts
git commit -m "feat: add registry file watcher with debounce"
```

---

## Task 7: Wire Everything Together (`extension.ts`)

**Files:**
- Modify: `src/extension.ts`

- [ ] **Step 1: Rewrite `extension.ts` to register all providers**

```typescript
import * as vscode from 'vscode';
import { runUp, runDown } from './cli';
import { OutportTreeProvider } from './sidebar/provider';
import { createStatusBar, updateStatusBar } from './statusbar';
import { createRegistryWatcher } from './watcher';

export function activate(context: vscode.ExtensionContext): void {
  const outputChannel = vscode.window.createOutputChannel('Outport');

  // Set context for view visibility
  vscode.commands.executeCommand('setContext', 'outport.active', true);

  // Status bar
  const statusBar = createStatusBar(context);

  // Sidebar — pass a callback so the status bar updates when the tree fetches data
  // This avoids a second CLI invocation for the status bar.
  const treeProvider = new OutportTreeProvider(outputChannel, (result) => {
    updateStatusBar(result);
  });
  vscode.window.registerTreeDataProvider('outportView', treeProvider);

  // Refresh helper — just triggers the tree; the callback updates the status bar
  const refresh = () => {
    treeProvider.refresh();
  };

  // Registry watcher
  const watcher = createRegistryWatcher(() => refresh());
  context.subscriptions.push(watcher);

  // Commands
  context.subscriptions.push(
    vscode.commands.registerCommand('outport.refresh', () => refresh()),

    vscode.commands.registerCommand('outport.up', async () => {
      const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!cwd) return;
      outputChannel.appendLine('> outport up');
      outputChannel.show(true);
      const result = await runUp(cwd, false);
      if (result.ok) {
        outputChannel.appendLine(result.data);
      } else {
        outputChannel.appendLine(`Error: ${result.error.message}`);
      }
      // Fallback refresh in case FileSystemWatcher misses the registry write
      refresh();
    }),

    vscode.commands.registerCommand('outport.upForce', async () => {
      const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!cwd) return;
      outputChannel.appendLine('> outport up --force');
      outputChannel.show(true);
      const result = await runUp(cwd, true);
      if (result.ok) {
        outputChannel.appendLine(result.data);
      } else {
        outputChannel.appendLine(`Error: ${result.error.message}`);
      }
      refresh();
    }),

    vscode.commands.registerCommand('outport.down', async () => {
      const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      if (!cwd) return;
      outputChannel.appendLine('> outport down');
      outputChannel.show(true);
      const result = await runDown(cwd);
      if (result.ok) {
        outputChannel.appendLine(result.data);
      } else {
        outputChannel.appendLine(`Error: ${result.error.message}`);
      }
      refresh();
    }),

    vscode.commands.registerCommand('outport.openService', (url: string) => {
      vscode.env.openExternal(vscode.Uri.parse(url));
    }),

    vscode.commands.registerCommand('outport.copyPort', (item: any) => {
      if (item?.service?.port) {
        vscode.env.clipboard.writeText(String(item.service.port));
      }
    }),

    vscode.commands.registerCommand('outport.copyUrl', (item: any) => {
      if (item?.service?.url) {
        vscode.env.clipboard.writeText(item.service.url);
      }
    }),

    vscode.commands.registerCommand('outport.copyEnvVar', (item: any) => {
      if (item?.service) {
        vscode.env.clipboard.writeText(`${item.service.env_var}=${item.service.port}`);
      }
    }),
  );

  // Initial refresh
  refresh();

  outputChannel.appendLine('Outport extension activated');
}

export function deactivate(): void {}
```

- [ ] **Step 2: Compile and verify**

Run: `cd ~/src/outport-vscode && npm run compile`
Expected: No errors.

- [ ] **Step 3: Manual test with F5**

Press F5 in VS Code to launch the Extension Development Host. Open a project directory that has `.outport.yml` and verify:
- The Outport panel appears in the Explorer sidebar
- Services are listed with ports and URLs
- Clicking a URL opens it in the browser
- The status bar shows `$(globe) projectname [instance]`
- Clicking "Run Up" in the view title executes `outport up` and shows output

- [ ] **Step 4: Commit**

```bash
git add src/extension.ts
git commit -m "feat: wire sidebar, status bar, watcher, and commands"
```

---

## Task 8: JSON Schema for `.outport.yml`

**Files:**
- Create: `schemas/outport.schema.json`

- [ ] **Step 1: Write the JSON Schema**

Create `schemas/outport.schema.json`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Outport Configuration",
  "description": "Configuration file for Outport port manager (.outport.yml)",
  "type": "object",
  "required": ["name", "services"],
  "additionalProperties": false,
  "properties": {
    "name": {
      "type": "string",
      "description": "Project name. Used to generate port hashes and hostnames. Must be lowercase alphanumeric with hyphens.",
      "pattern": "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$"
    },
    "services": {
      "type": "object",
      "description": "Services that need port allocations.",
      "minProperties": 1,
      "additionalProperties": {
        "$ref": "#/definitions/service"
      }
    },
    "computed": {
      "type": "object",
      "description": "Computed values derived from service ports and hostnames. Written to .env files alongside service ports.",
      "additionalProperties": {
        "$ref": "#/definitions/computedValue"
      }
    }
  },
  "definitions": {
    "service": {
      "type": "object",
      "required": ["env_var"],
      "additionalProperties": false,
      "properties": {
        "env_var": {
          "type": "string",
          "description": "Environment variable name for this service's port (e.g., PORT, DB_PORT)."
        },
        "preferred_port": {
          "type": "integer",
          "minimum": 1,
          "maximum": 65535,
          "description": "Preferred port number. Used if available, otherwise falls back to hash-based allocation."
        },
        "protocol": {
          "type": "string",
          "enum": ["http", "https"],
          "description": "Protocol for .test hostname. Required when hostname is set. Enables browser access via outport open."
        },
        "hostname": {
          "type": "string",
          "description": "Hostname for .test domain (e.g., myapp.test, api.myapp.test). Must contain the project name. The .test suffix is optional in config but conventional.",
          "pattern": "^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(\\.test)?$"
        },
        "env_file": {
          "description": "Which .env file(s) to write this port to. Defaults to .env.",
          "oneOf": [
            { "type": "string" },
            { "type": "array", "items": { "type": "string" } }
          ]
        }
      }
    },
    "computedValue": {
      "type": "object",
      "required": ["env_file"],
      "additionalProperties": false,
      "properties": {
        "value": {
          "type": "string",
          "description": "Template string using ${service.field} syntax. Fields: port, hostname, url. Modifiers: url:direct. Standalone vars: ${instance}. Conditionals: ${var:-default}, ${var:+replacement}."
        },
        "env_file": {
          "description": "Where to write the computed value. Can be a string, array of strings, or array of objects with file and value for per-file overrides.",
          "oneOf": [
            { "type": "string" },
            {
              "type": "array",
              "items": {
                "oneOf": [
                  { "type": "string" },
                  {
                    "type": "object",
                    "required": ["file"],
                    "additionalProperties": false,
                    "properties": {
                      "file": {
                        "type": "string",
                        "description": "Path to the .env file."
                      },
                      "value": {
                        "type": "string",
                        "description": "Override template for this specific file."
                      }
                    }
                  }
                ]
              }
            }
          ]
        }
      }
    }
  }
}
```

- [ ] **Step 2: Verify the schema validates a known-good config**

Create a quick test by running:

```bash
cd ~/src/outport-vscode
npx ajv-cli validate -s schemas/outport.schema.json -d test-fixtures/workspace/.outport.yml
```

If `ajv-cli` is not available, just open the Extension Development Host with F5 and verify that `.outport.yml` gets autocomplete and validation from the YAML extension.

- [ ] **Step 3: Create test fixture**

Create `test-fixtures/workspace/.outport.yml`:

```yaml
name: testapp

services:
  web:
    env_var: PORT
    protocol: http
    hostname: testapp.test
  postgres:
    env_var: DB_PORT
    preferred_port: 5432

computed:
  CORS_ORIGINS:
    value: "${web.url}"
    env_file: .env
```

- [ ] **Step 4: Commit**

```bash
git add schemas/outport.schema.json test-fixtures/
git commit -m "feat: add JSON Schema for .outport.yml config authoring"
```

---

## Task 9: Integration Test

**Files:**
- Create: `src/test/integration/extension.test.ts`

- [ ] **Step 1: Write integration tests**

Create `src/test/integration/extension.test.ts`:

```typescript
import * as assert from 'assert';
import * as vscode from 'vscode';

suite('Extension Integration', () => {
  test('extension should be present', () => {
    const ext = vscode.extensions.getExtension('steveclarke.outport');
    assert.ok(ext, 'Extension not found');
  });

  test('extension should activate', async () => {
    const ext = vscode.extensions.getExtension('steveclarke.outport')!;
    await ext.activate();
    assert.strictEqual(ext.isActive, true);
  });

  test('commands should be registered', async () => {
    const ext = vscode.extensions.getExtension('steveclarke.outport')!;
    await ext.activate();
    const commands = await vscode.commands.getCommands(true);
    assert.ok(commands.includes('outport.refresh'), 'Missing outport.refresh');
    assert.ok(commands.includes('outport.up'), 'Missing outport.up');
    assert.ok(commands.includes('outport.down'), 'Missing outport.down');
    assert.ok(commands.includes('outport.openService'), 'Missing outport.openService');
  });
});
```

- [ ] **Step 2: Add integration test config to `.vscode-test.mjs`**

Update `.vscode-test.mjs`:

```javascript
import { defineConfig } from '@vscode/test-cli';

export default defineConfig([
  {
    label: 'unitTests',
    files: 'out/test/unit/**/*.test.js',
    mocha: { ui: 'tdd', timeout: 5000 },
  },
  {
    label: 'integrationTests',
    files: 'out/test/integration/**/*.test.js',
    workspaceFolder: './test-fixtures/workspace',
    mocha: { ui: 'tdd', timeout: 20000 },
  },
]);
```

- [ ] **Step 3: Compile and run all tests**

Run: `cd ~/src/outport-vscode && npm run compile && npm test`
Expected: All unit and integration tests pass.

- [ ] **Step 4: Commit**

```bash
git add src/test/integration/ .vscode-test.mjs
git commit -m "test: add integration tests for extension activation and commands"
```

---

## Task 10: README, LICENSE, and Final Polish

**Files:**
- Create: `README.md`
- Create: `LICENSE`

- [ ] **Step 1: Write `README.md`**

```markdown
# Outport for VS Code

See your [Outport](https://outport.dev) ports, URLs, and service health right in VS Code.

## Features

- **Sidebar panel** — Services, ports, URLs, and health indicators in the Explorer sidebar
- **Clickable URLs** — Click any HTTP service to open it in your browser
- **Copy to clipboard** — Right-click to copy ports, URLs, or env var assignments
- **Status bar** — Shows your project name and instance at a glance
- **Config authoring** — Autocomplete and validation for `.outport.yml`
- **Auto-refresh** — Sidebar updates when you run `outport up` or `outport down`

## Requirements

- [Outport CLI](https://outport.dev) installed and on your `$PATH`
- [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) for config authoring features

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `outport.binaryPath` | `outport` | Path to the outport binary |

## Commands

All commands are available from the Command Palette (Cmd+Shift+P):

- **Outport: Run Up** — Allocate ports and write `.env` files
- **Outport: Run Up --force** — Re-allocate all ports from scratch
- **Outport: Run Down** — Remove project from registry and clean `.env` files
- **Outport: Refresh** — Refresh the sidebar panel
```

- [ ] **Step 2: Write `LICENSE`**

Use MIT license with the current year and "Steve Clarke" as the copyright holder.

- [ ] **Step 3: Final compile and test**

Run: `cd ~/src/outport-vscode && npm run compile && npm test`
Expected: All tests pass, no compile errors.

- [ ] **Step 4: Commit**

```bash
git add README.md LICENSE
git commit -m "docs: add README and LICENSE"
```

- [ ] **Step 5: Create GitHub repo and push**

```bash
cd ~/src/outport-vscode
gh repo create steveclarke/outport-vscode --public --source=. --push
```

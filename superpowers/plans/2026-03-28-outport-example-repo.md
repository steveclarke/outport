# outport-example Repository — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone reference repository demonstrating Outport's port orchestration across a multi-service Node/Express stack.

**Architecture:** Four bare Express services (api, app, admin, docs) in npm workspaces. Each service is ~30 lines — loads dotenv, serves its env vars as JSON or HTML. Outport wires everything together via `outport.yml` computed values. Process-compose orchestrates startup with health checks.

**Tech Stack:** Node.js, Express, dotenv, process-compose, Bruno

**Target directory:** New repo at `~/src/outport-example`

---

## File Map

| File | Responsibility |
|------|---------------|
| `package.json` | Root npm workspace definition |
| `outport.yml` | Port allocation and cross-service env var wiring |
| `outport.local.yml` | Commented-out local overrides example |
| `process-compose.yml` | Service orchestration with health checks |
| `.gitignore` | Exclude generated `.env`, `.pc_env`, `node_modules/` |
| `api/package.json` | API service dependencies (express, dotenv) |
| `api/server.js` | API server — JSON endpoint with env vars, `/health` |
| `app/package.json` | App frontend dependencies |
| `app/server.js` | App server — HTML page showing env vars |
| `admin/package.json` | Admin frontend dependencies |
| `admin/server.js` | Admin server — HTML page showing env vars |
| `docs/package.json` | Docs service dependencies |
| `docs/server.js` | Docs server — HTML page showing env vars |
| `docker/compose.yml` | Placeholder Docker Compose with `COMPOSE_PROJECT_NAME` |
| `bruno/bruno.json` | Bruno collection config |
| `bruno/environments/dev.bru` | Bruno dev environment reading from env vars |
| `bruno/api/status.bru` | Bruno request — GET API status |
| `bin/start-api` | Shell script to start API |
| `bin/start-app` | Shell script to start app |
| `bin/start-admin` | Shell script to start admin |
| `bin/start-docs` | Shell script to start docs |
| `bin/test-api` | Curl script to probe all services |
| `README.md` | Setup guide, walkthrough, worktree demo |

---

### Task 1: Initialize repo and root config

**Files:**
- Create: `~/src/outport-example/package.json`
- Create: `~/src/outport-example/.gitignore`
- Create: `~/src/outport-example/outport.yml`
- Create: `~/src/outport-example/outport.local.yml`

- [ ] **Step 1: Create repo directory and initialize git**

```bash
mkdir ~/src/outport-example
cd ~/src/outport-example
git init
```

- [ ] **Step 2: Create root package.json**

```json
{
  "name": "outport-example",
  "private": true,
  "workspaces": ["api", "app", "admin", "docs"],
  "scripts": {
    "setup": "npm install"
  }
}
```

- [ ] **Step 3: Create .gitignore**

```
**/.env
.pc_env
node_modules/
```

- [ ] **Step 4: Create outport.yml**

```yaml
name: example

services:
  api:
    env_var: API_PORT
    hostname: api.example
    env_file:
      - api/.env
  app:
    env_var: APP_PORT
    hostname: example
    env_file:
      - app/.env
  admin:
    env_var: ADMIN_PORT
    hostname: admin.example
    env_file:
      - admin/.env
  docs:
    env_var: DOCS_PORT
    hostname: docs.example
    env_file:
      - docs/.env

computed:
  # Frontend → API wiring (server-to-server, use :direct)
  API_URL:
    value: "${api.url:direct}"
    env_file:
      - app/.env
      - admin/.env

  # Frontend → API wiring (browser-facing, for display)
  API_HOSTNAME:
    value: "${api.url}"
    env_file:
      - app/.env
      - admin/.env

  # API → Frontend wiring (browser-facing, for CORS)
  CORS_ORIGINS:
    value: "${app.url},${admin.url}"
    env_file: api/.env

  # Docker Compose isolation per worktree
  COMPOSE_PROJECT_NAME:
    value: "${project_name}${instance:+-${instance}}"
    env_file: docker/.env

  # Process-compose integration
  HEALTH_CHECK_URL:
    value: "${api.url:direct}/health"
    env_file: .pc_env

  PC_SOCKET_PATH:
    value: "/tmp/process-compose-${project_name}${instance:+-${instance}}.sock"
    env_file: .pc_env

  # Bruno API testing (server-to-server)
  BRUNO_API_URL:
    value: "${api.url:direct}"
    env_file: bruno/.env
```

- [ ] **Step 5: Create outport.local.yml**

```yaml
# Local overrides — uncomment to customize for your environment.
# This file is for your machine only (gitignored in real projects).
#
# Only open the app and API when running `outport open`:
# open:
#   - app
#   - api
```

- [ ] **Step 6: Commit**

```bash
git add package.json .gitignore outport.yml outport.local.yml
git commit -m "feat: initialize repo with outport config and root workspace"
```

---

### Task 2: API service

**Files:**
- Create: `api/package.json`
- Create: `api/server.js`

- [ ] **Step 1: Create api/package.json**

```json
{
  "name": "api",
  "private": true,
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "dotenv": "^16.4.7",
    "express": "^4.21.2"
  }
}
```

- [ ] **Step 2: Create api/server.js**

The API server loads dotenv, starts Express on `API_PORT`, and serves two endpoints:
- `GET /` — returns JSON with all Outport-managed env vars
- `GET /health` — returns `{ "status": "ok" }` for process-compose health checks

```js
require("dotenv").config();
const express = require("express");

const app = express();
const port = process.env.API_PORT;

const envVars = {
  API_PORT: process.env.API_PORT,
  CORS_ORIGINS: process.env.CORS_ORIGINS,
  COMPOSE_PROJECT_NAME: process.env.COMPOSE_PROJECT_NAME,
};

app.get("/", (_req, res) => {
  res.json({
    service: "api",
    hostname: "api.example.test",
    port,
    env: envVars,
  });
});

app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

app.listen(port, () => {
  console.log(`API server listening on port ${port}`);
});
```

- [ ] **Step 3: Commit**

```bash
git add api/
git commit -m "feat: add API service"
```

---

### Task 3: App frontend service

**Files:**
- Create: `app/package.json`
- Create: `app/server.js`

- [ ] **Step 1: Create app/package.json**

```json
{
  "name": "app",
  "private": true,
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "dotenv": "^16.4.7",
    "express": "^4.21.2"
  }
}
```

- [ ] **Step 2: Create app/server.js**

The app server loads dotenv, starts Express on `APP_PORT`, and serves an HTML page showing its env vars. The page has minimal inline styling — clean, readable, not ugly.

```js
require("dotenv").config();
const express = require("express");

const app = express();
const port = process.env.APP_PORT;

const envVars = {
  APP_PORT: process.env.APP_PORT,
  API_URL: process.env.API_URL,
  API_HOSTNAME: process.env.API_HOSTNAME,
};

app.get("/", (_req, res) => {
  res.send(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>App — outport-example</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 0 20px; color: #1a1a1a; }
    h1 { font-size: 1.4rem; }
    table { border-collapse: collapse; width: 100%; margin-top: 16px; }
    td, th { text-align: left; padding: 8px 12px; border-bottom: 1px solid #e0e0e0; }
    th { color: #666; font-weight: 500; }
    code { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; font-size: 0.9em; }
  </style>
</head>
<body>
  <h1>App Frontend</h1>
  <p>Hostname: <code>example.test</code></p>
  <table>
    <tr><th>Variable</th><th>Value</th></tr>
    ${Object.entries(envVars).map(([k, v]) => `<tr><td><code>${k}</code></td><td><code>${v || "(not set)"}</code></td></tr>`).join("\n    ")}
  </table>
</body>
</html>`);
});

app.listen(port, () => {
  console.log(`App frontend listening on port ${port}`);
});
```

- [ ] **Step 3: Commit**

```bash
git add app/
git commit -m "feat: add app frontend service"
```

---

### Task 4: Admin frontend service

**Files:**
- Create: `admin/package.json`
- Create: `admin/server.js`

- [ ] **Step 1: Create admin/package.json**

```json
{
  "name": "admin",
  "private": true,
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "dotenv": "^16.4.7",
    "express": "^4.21.2"
  }
}
```

- [ ] **Step 2: Create admin/server.js**

Same pattern as app, different service name and env vars.

```js
require("dotenv").config();
const express = require("express");

const app = express();
const port = process.env.ADMIN_PORT;

const envVars = {
  ADMIN_PORT: process.env.ADMIN_PORT,
  API_URL: process.env.API_URL,
  API_HOSTNAME: process.env.API_HOSTNAME,
};

app.get("/", (_req, res) => {
  res.send(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Admin — outport-example</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 0 20px; color: #1a1a1a; }
    h1 { font-size: 1.4rem; }
    table { border-collapse: collapse; width: 100%; margin-top: 16px; }
    td, th { text-align: left; padding: 8px 12px; border-bottom: 1px solid #e0e0e0; }
    th { color: #666; font-weight: 500; }
    code { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; font-size: 0.9em; }
  </style>
</head>
<body>
  <h1>Admin Frontend</h1>
  <p>Hostname: <code>admin.example.test</code></p>
  <table>
    <tr><th>Variable</th><th>Value</th></tr>
    ${Object.entries(envVars).map(([k, v]) => `<tr><td><code>${k}</code></td><td><code>${v || "(not set)"}</code></td></tr>`).join("\n    ")}
  </table>
</body>
</html>`);
});

app.listen(port, () => {
  console.log(`Admin frontend listening on port ${port}`);
});
```

- [ ] **Step 3: Commit**

```bash
git add admin/
git commit -m "feat: add admin frontend service"
```

---

### Task 5: Docs service

**Files:**
- Create: `docs/package.json`
- Create: `docs/server.js`

- [ ] **Step 1: Create docs/package.json**

```json
{
  "name": "docs",
  "private": true,
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "dotenv": "^16.4.7",
    "express": "^4.21.2"
  }
}
```

- [ ] **Step 2: Create docs/server.js**

Same pattern. Docs has fewer env vars — just its own port.

```js
require("dotenv").config();
const express = require("express");

const app = express();
const port = process.env.DOCS_PORT;

const envVars = {
  DOCS_PORT: process.env.DOCS_PORT,
};

app.get("/", (_req, res) => {
  res.send(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Docs — outport-example</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 0 20px; color: #1a1a1a; }
    h1 { font-size: 1.4rem; }
    table { border-collapse: collapse; width: 100%; margin-top: 16px; }
    td, th { text-align: left; padding: 8px 12px; border-bottom: 1px solid #e0e0e0; }
    th { color: #666; font-weight: 500; }
    code { background: #f0f0f0; padding: 2px 6px; border-radius: 3px; font-size: 0.9em; }
  </style>
</head>
<body>
  <h1>Docs</h1>
  <p>Hostname: <code>docs.example.test</code></p>
  <table>
    <tr><th>Variable</th><th>Value</th></tr>
    ${Object.entries(envVars).map(([k, v]) => `<tr><td><code>${k}</code></td><td><code>${v || "(not set)"}</code></td></tr>`).join("\n    ")}
  </table>
</body>
</html>`);
});

app.listen(port, () => {
  console.log(`Docs server listening on port ${port}`);
});
```

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "feat: add docs service"
```

---

### Task 6: bin/ scripts

**Files:**
- Create: `bin/start-api`
- Create: `bin/start-app`
- Create: `bin/start-admin`
- Create: `bin/start-docs`
- Create: `bin/test-api`

- [ ] **Step 1: Create bin/start-api**

```bash
#!/usr/bin/env bash
cd "$(dirname "$0")/../api" && npm start
```

- [ ] **Step 2: Create bin/start-app**

```bash
#!/usr/bin/env bash
cd "$(dirname "$0")/../app" && npm start
```

- [ ] **Step 3: Create bin/start-admin**

```bash
#!/usr/bin/env bash
cd "$(dirname "$0")/../admin" && npm start
```

- [ ] **Step 4: Create bin/start-docs**

```bash
#!/usr/bin/env bash
cd "$(dirname "$0")/../docs" && npm start
```

- [ ] **Step 5: Create bin/test-api**

This script sources each service's `.env` and curls its root endpoint. Uses `outport-example/` as context — must be run from the repo root.

```bash
#!/usr/bin/env bash
set -e

echo "=== API (api.example.test) ==="
source api/.env
curl -s "http://localhost:${API_PORT}/"
echo -e "\n"

echo "=== App (example.test) ==="
source app/.env
curl -s "http://localhost:${APP_PORT}/" | head -5
echo -e "\n"

echo "=== Admin (admin.example.test) ==="
source admin/.env
curl -s "http://localhost:${ADMIN_PORT}/" | head -5
echo -e "\n"

echo "=== Docs (docs.example.test) ==="
source docs/.env
curl -s "http://localhost:${DOCS_PORT}/" | head -5
echo
```

- [ ] **Step 6: Make all bin scripts executable**

```bash
chmod +x bin/*
```

- [ ] **Step 7: Commit**

```bash
git add bin/
git commit -m "feat: add bin scripts for starting services and testing"
```

---

### Task 7: Process-compose config

**Files:**
- Create: `process-compose.yml`

- [ ] **Step 1: Create process-compose.yml**

```yaml
version: "0.5"

shell:
  shell_command: "bash"
  shell_argument: "-lc"

processes:
  api:
    command: bin/start-api
    readiness_probe:
      exec:
        command: "curl -sf ${HEALTH_CHECK_URL}"
      initial_delay_seconds: 1
      period_seconds: 3
      failure_threshold: 5

  app:
    command: bin/start-app
    depends_on:
      api:
        condition: process_healthy

  admin:
    command: bin/start-admin
    depends_on:
      api:
        condition: process_healthy

  docs:
    command: bin/start-docs
```

- [ ] **Step 2: Commit**

```bash
git add process-compose.yml
git commit -m "feat: add process-compose orchestration"
```

---

### Task 8: Docker Compose placeholder

**Files:**
- Create: `docker/compose.yml`

- [ ] **Step 1: Create docker/compose.yml**

```yaml
# This file demonstrates how Outport isolates Docker Compose
# across worktrees via COMPOSE_PROJECT_NAME.
#
# The .env file in this directory is generated by `outport up`
# and contains COMPOSE_PROJECT_NAME=example (or example-xbjf
# for worktrees).
#
# If you have Docker installed, `docker compose up` works.
# If not, the rest of the example still runs fine.

services:
  db:
    image: postgres:17
    environment:
      POSTGRES_PASSWORD: postgres
```

- [ ] **Step 2: Commit**

```bash
git add docker/
git commit -m "feat: add Docker Compose placeholder showing COMPOSE_PROJECT_NAME isolation"
```

---

### Task 9: Bruno collection

**Files:**
- Create: `bruno/bruno.json`
- Create: `bruno/environments/dev.bru`
- Create: `bruno/api/status.bru`

- [ ] **Step 1: Create bruno/bruno.json**

```json
{
  "version": "1",
  "name": "outport-example",
  "type": "collection",
  "ignore": ["node_modules", ".git"]
}
```

- [ ] **Step 2: Create bruno/environments/dev.bru**

```
vars {
  url: {{process.env.BRUNO_API_URL}}
}
```

- [ ] **Step 3: Create bruno/api/status.bru**

```
meta {
  name: Status
  type: http
  seq: 1
}

get {
  url: {{url}}/
  body: none
  auth: none
}
```

- [ ] **Step 4: Commit**

```bash
git add bruno/
git commit -m "feat: add Bruno API collection wired to outport env vars"
```

---

### Task 10: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md**

The README walks through the complete developer experience. Sections:

1. **What is this?** — One paragraph. Reference repo showing how Outport orchestrates ports, hostnames, and env vars across a multi-service dev environment. Four Node/Express services, zero configuration — just `outport up`.

2. **Prerequisites**
   - Required: Node.js 18+, [Outport](https://outport.dev)
   - Optional: [process-compose](https://f1bonacc1.github.io/process-compose/) (for one-command startup), [Bruno](https://www.usebruno.com/) (for API testing), Docker (for the compose isolation demo)

3. **Quick start**
   ```
   git clone https://github.com/steveclarke/outport-example.git
   cd outport-example
   outport setup && outport up
   npm install
   process-compose up
   ```
   Visit https://outport.test to see the dashboard. Click through to each service.

4. **The manual way** — For those without process-compose. Run each `bin/start-*` in a separate terminal. Point out this is exactly what process-compose automates.

5. **What just happened?** — Walk through what `outport up` did:
   - Allocated deterministic ports for each service
   - Computed cross-service URLs (API URL for frontends, CORS origins for the API)
   - Wrote everything to the right `.env` files
   - Show example contents of `api/.env` and `app/.env` side by side

6. **Explore**
   - Visit each `.test` hostname in the browser
   - Run `bin/test-api` to probe services from the command line
   - Open the `bruno/` collection in Bruno for GUI-based API testing

7. **Try worktrees** — The isolation proof:
   ```
   git worktree add ../outport-example-feature feature-branch
   cd ../outport-example-feature
   outport up
   ```
   Different ports, different `COMPOSE_PROJECT_NAME`, same config file. Show the contrast.

8. **Local overrides** — Mention `outport.local.yml` is already in the repo with a commented-out example. Uncomment the `open` list to customize which services `outport open` launches.

9. **Learn more** — Links to:
   - https://outport.dev — Outport docs
   - https://outport.dev/guide/configuration — Config reference
   - https://f1bonacc1.github.io/process-compose/ — Process-compose docs

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with setup guide and walkthrough"
```

---

### Task 11: Install, run outport up, and verify

This is the smoke test. Run the full setup and confirm everything works end-to-end.

- [ ] **Step 1: Install dependencies**

```bash
cd ~/src/outport-example
npm install
```

- [ ] **Step 2: Run outport setup and outport up**

```bash
outport setup
outport up
```

Verify: `.env` files created in `api/`, `app/`, `admin/`, `docs/`, `docker/`, `bruno/`, and `.pc_env` at root. Each should contain the expected port and computed values.

- [ ] **Step 3: Start services and verify**

```bash
process-compose up
```

In another terminal:

```bash
cd ~/src/outport-example
bin/test-api
```

Verify: API returns JSON with env vars. App/admin/docs return HTML.

- [ ] **Step 4: Visit in browser**

Open https://outport.test — verify dashboard shows all 4 services. Click through to each `.test` hostname. Confirm each page displays its env vars correctly.

- [ ] **Step 5: Verify Bruno collection**

Open `bruno/` directory in Bruno. Switch to `dev` environment. Run the `Status` request. Verify it returns the API's env var JSON.

# Docs Round 2 Design Spec

**Date:** 2026-03-18
**Status:** Approved

---

## Changes

### 1. New page: `docs/guide/why-outport.md`

Add to sidebar nav between "Examples" and "VS Code Extension".

**Structure:**

#### Section: "The Problem"

3-4 sentences painting the pain. Adapted from the adversarial review's narrative:

> You're running Rails on 3000, a Nuxt frontend on 5173, Postgres on 5432. You start a second project — port conflict. You switch to a worktree — same ports. You change a port, and now your CORS config, API URLs, and Docker Compose port mappings are all wrong.
>
> Most developers solve this with a mental spreadsheet and `kill -9`. It works until it doesn't.

#### Section: "How Outport Compares"

Markdown comparison table:

| Feature | Outport | Portless (Vercel) | puma-dev / Valet | Docker Compose | mkcert + Caddy |
|---|---|---|---|---|---|
| Deterministic ports | Yes | No (random) | No | Manual | No |
| Multi-service config | Yes | One app at a time | One app at a time | Yes | Manual per project |
| .env generation | Yes (fenced blocks) | No | No | No | No |
| Computed values | Yes (`${service.url}`) | No | No | No | No |
| .test hostnames | Yes | .localhost | Yes | No | Yes (manual config) |
| Auto HTTPS | Yes | Yes | Yes | No | Yes |
| Multi-instance / worktrees | Yes | Detects worktrees | No | No | No |
| Language-agnostic | Yes | JS-focused | Ruby / PHP | Yes | Yes |

#### Section: "When to Use What"

Short, honest positioning — no bashing:

- **Docker Compose** — Great if your team is already containerized. Outport complements Docker by managing host-side port mappings and `.env` files.
- **Portless** — Solid for single-app workflows. Outport is designed for multi-service projects with cross-service URL wiring.
- **puma-dev / Valet** — Good for `.test` domains in Ruby/PHP. Outport adds port orchestration, `.env` generation, and works with any language.
- **mkcert + Caddy** — Handles local HTTPS well. Outport bundles that functionality and adds port allocation, computed values, and multi-instance support.

### 2. Homepage AI/worktree callout

In `docs/.vitepress/theme/HomeLayout.vue`, add a brief line after the terminal demo pair (after the closing `</div>` of `.terminal-pair`, still inside the hero section):

```html
<p class="hero-callout">
  Built for parallel development — worktrees, AI agents, and multiple checkouts.
  <a href="/guide/work-with-ai">Learn more →</a>
</p>
```

Styled as centered secondary text with the link in brand color.

### 3. macOS-only notice on homepage

In the install section, add a note below the `brew install` pill:

```html
<p class="install-note">macOS only. Linux support is experimental.</p>
```

Styled in `var(--vp-c-text-3)`, small font, centered.

---

## Files Changed

| File | Change |
|------|--------|
| `docs/guide/why-outport.md` | New page |
| `docs/.vitepress/config.ts` | Add sidebar entry |
| `docs/.vitepress/theme/HomeLayout.vue` | AI/worktree callout + macOS notice |

## Validation

- `npm run docs:build` succeeds
- New page renders correctly with table
- Sidebar shows "Why Outport?" in correct position
- Homepage callout and notice render correctly

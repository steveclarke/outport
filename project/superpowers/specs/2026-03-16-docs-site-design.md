# Outport Documentation Site Design

**Date:** 2026-03-16
**Issue:** #30

## Overview

Build the outport.dev documentation site using VitePress. Landing page with brand identity + essential documentation pages. Deployed to Cloudflare Pages.

## Decisions

- **Location:** `docs/` directory (VitePress convention). Existing `docs/specs/` and `docs/superpowers/` move to `project/specs/` and `project/superpowers/` respectively (alongside existing files in `project/`).
- **Deployment:** Cloudflare Pages with git-integrated deploys.
- **Scope:** Landing page + essentials (getting started, installation, configuration reference, commands reference). Additional guides (instance management, template system, integrations) are follow-up work.
- **Theme:** VitePress DefaultTheme with CSS variable overrides for brand colors/fonts. Custom `HomeLayout.vue` for the landing page, rendered within the default theme shell (VitePress nav bar persists). Registered in `index.ts` by overriding the `home` layout slot so `index.md` can use `layout: home` frontmatter.
- **Color palette:** Warm cream background (`#faf8f5`) with deep cream sections (`#f5f0e8`), navy headings (`#031C54`), steel blue accents (`#2E86AB`), cream border (`#e8e3da`).
- **Typography:** Barlow Bold 700 (headings/wordmark) + Inter Regular 400 / Medium 500 (body), loaded from local font files via `@font-face`.
- **Dark mode:** Disabled (`appearance: false` in config). Cream palette has no dark variants — this avoids a broken toggle. Dark mode is follow-up work.
- **Clean URLs:** Enabled (`cleanUrls: true`) for paths like `/guide/getting-started` without `.html`.
- **VitePress version:** Pin to `^1.6`.
- **Domain:** `outport.dev` is the docs site domain. Existing `outport.app` references in the codebase (README, `.goreleaser.yml`, `cmd/init.go`) are a separate cleanup — out of scope for this issue.

## Site Structure

```
package.json                 # Root-level: VitePress dep + scripts (docs:dev, docs:build, docs:preview)
package-lock.json            # Committed for reproducible builds (especially Cloudflare deploys)
docs/
├── .vitepress/
│   ├── config.ts            # VitePress config (nav, sidebar, head, meta)
│   ├── theme/
│   │   ├── index.ts         # Extend DefaultTheme, override home layout slot with HomeLayout
│   │   ├── custom.css       # Brand colors, fonts, warm cream overrides
│   │   └── HomeLayout.vue   # Custom landing page content (rendered inside theme shell with nav)
│   └── public/
│       ├── fonts/
│       │   ├── Barlow-Bold.ttf   # Copied from brand/fonts/
│       │   └── Inter.ttf         # Copied from brand/fonts/ (variable font, weights 400-700)
│       ├── logo-horizontal-color.svg  # Copied from brand/svg/ (nav logo)
│       ├── mark-color.svg             # Copied from brand/svg/ (small contexts)
│       ├── og-image-1280x640.png      # Copied from brand/social/
│       ├── favicon.ico                # Copied from brand/favicon/
│       ├── favicon-16x16.png          # Copied from brand/favicon/
│       ├── favicon-32x32.png          # Copied from brand/favicon/
│       ├── apple-touch-icon.png       # Copied from brand/favicon/
│       ├── android-chrome-192x192.png # Copied from brand/favicon/
│       ├── android-chrome-512x512.png # Copied from brand/favicon/
│       └── site.webmanifest           # Copied from brand/favicon/ (paths already correct)
├── index.md                 # Landing page (layout: home → renders HomeLayout via slot override)
├── guide/
│   ├── getting-started.md   # init → apply → setup walkthrough
│   └── installation.md      # Homebrew, from source, local build
└── reference/
    ├── configuration.md     # .outport.yml schema with annotated examples
    └── commands.md          # All CLI commands, grouped logically
```

Brand assets in `docs/.vitepress/public/` are copies from `brand/`. The `brand/` directory remains the canonical source. If brand assets change, the copies need updating manually (no build-step sync).

## Landing Page Sections

`HomeLayout.vue` renders within the VitePress default theme shell — the nav bar, search, and social links are provided by VitePress. `HomeLayout.vue` contains only the page content below the nav.

Top to bottom, modular self-contained sections. Each is a `<section>` block that can be reordered independently.

The hero terminal demo is a static styled `<pre>` block (not animated, not a screenshot).

1. **Hero** — "Stop fighting port conflicts" headline (Barlow 700, navy). Subtitle explaining what Outport does. Two CTAs: "Get Started" (steel blue primary) + "View on GitHub" (outlined secondary). Static terminal block showing:
   ```
   $ outport apply

   myapp · main

     service    port   hostname
     rails      13842  myapp.test
     postgres   28391
     redis      19204

   → .env updated
   ```
2. **Feature grid** — 2x2 card grid on white cards with cream border:
   - Deterministic Ports — same inputs, same ports, always
   - .test Domains — real hostnames, built-in DNS and reverse proxy
   - Multi-Instance — worktrees/branches get their own ports
   - .env Integration — ports and URLs written to env files
3. **How it works** — Three-step flow: Configure (.outport.yml), Apply (outport apply), Develop (.env has everything). Each step shows a code/terminal snippet.
4. **Install** — "Get started in seconds" with `brew install steveclarke/tap/outport` in a dark terminal block.
5. **Footer** — MIT license, GitHub link, author credit.

## Documentation Pages

### Getting Started (`guide/getting-started.md`)

- Prerequisites (macOS, Homebrew or Go)
- Install Outport
- `outport init` — walk through creating `.outport.yml` with a real example
- `outport apply` — show the output, explain what happened (registry, .env block)
- `outport setup` — enable .test domains (DNS + proxy daemon)
- "What just happened" — brief explanation of registry, fenced .env blocks, DNS resolution

### Installation (`guide/installation.md`)

- Homebrew (primary): `brew install steveclarke/tap/outport`
- From source: `go install github.com/outport-app/outport@latest`
- Local build: `just build && just install`

### Configuration Reference (`reference/configuration.md`)

- Full `.outport.yml` schema with annotated example
- `name` field (required, naming rules)
- `services` map: `env_var`, `env_file` (string or array), `preferred_port`, `protocol`, `hostname`
- Derived values: `${service.field}` syntax, `${service.url:direct}` modifier
- Multiple env files for monorepos
- Content adapted from existing `project/specs/configuration.md` (after move)

### Commands Reference (`reference/commands.md`)

- All user-facing commands with synopsis, flags, example output
- Hidden commands (`daemon`) excluded — it's invoked by launchd, not users
- Grouped logically:
  - **Core:** init, apply, unapply, ports
  - **Navigation:** open, status
  - **Maintenance:** gc, rename, promote
  - **Daemon:** setup, teardown, up, down
- `--json` flag documented for each command
- `--force` flag on apply

## VitePress Configuration

### `config.ts`

- `title`: "Outport"
- `description`: "Deterministic port management for multi-project development"
- `appearance`: false (disable dark mode toggle)
- `cleanUrls`: true
- `head`: favicon links, OG meta tags, font preloads
- `themeConfig.logo`: `/logo-horizontal-color.svg`
- `themeConfig.nav`: Guide, Reference, GitHub link
- `themeConfig.sidebar`: Guide section (Getting Started, Installation), Reference section (Configuration, Commands)
- `themeConfig.socialLinks`: GitHub repo
- `themeConfig.search`: local search (VitePress built-in)

### `custom.css`

Brand color overrides via VitePress CSS variables:
- `--vp-c-brand-1`: `#2E86AB` (steel blue)
- `--vp-c-bg`: `#faf8f5` (warm cream)
- `--vp-c-bg-soft`: `#f5f0e8` (deep cream)
- `--vp-c-divider`: `#e8e3da` (cream border)
- `--vp-c-text-1`: `#1e293b` (slate-900)
- `@font-face` for Barlow Bold 700 (`/fonts/Barlow-Bold.ttf`)
- `@font-face` for Inter variable (`/fonts/Inter.ttf`, weight range 400-700)
- Heading font-family override to Barlow

### Root `package.json`

```json
{
  "private": true,
  "scripts": {
    "docs:dev": "vitepress dev docs",
    "docs:build": "vitepress build docs",
    "docs:preview": "vitepress preview docs"
  },
  "devDependencies": {
    "vitepress": "^1.6"
  }
}
```

`package-lock.json` is committed for reproducible builds.

## File Moves

Before VitePress setup, move existing internal docs:
- `docs/specs/` → `project/specs/`
- `docs/superpowers/` → `project/superpowers/`

Existing files in `project/` (`vision.md`, `releasing.md`, `research.md`, `design-challenge-monorepo.md`) remain in place.

## Gitignore Additions

Add to `.gitignore`:
- `node_modules/`
- `docs/.vitepress/dist/`
- `docs/.vitepress/cache/`

## Just Recipes

Add to `justfile`:
- `docs` — `npm run docs:dev` (start dev server). Note: requires `npm install` first.
- `docs-build` — `npm run docs:build` (production build)
- `docs-preview` — `npm run docs:preview` (preview production build)

## Deployment

Cloudflare Pages configured via the dashboard:
- Install command: `npm ci`
- Build command: `npm run docs:build`
- Build output: `docs/.vitepress/dist`
- Production branch: `master`
- Node.js version: Set `NODE_VERSION=20` environment variable
- Custom domain: `outport.dev`
- Preview deploys on PRs are enabled by default (Cloudflare Pages feature)

## Modularity

All landing page sections are self-contained within `HomeLayout.vue`. Sections can be reordered by moving `<section>` blocks. Documentation sidebar order is controlled entirely by the `sidebar` config in `config.ts`. Adding new pages is: create `.md` file, add to sidebar config.

## Out of Scope (follow-up issues)

- Instance management guide
- Template system / derived values guide
- Integration guides (Docker, Rails, Nuxt, Phoenix, Django)
- .test domains deep-dive
- FAQ page
- Dark mode (needs dark variants of cream palette)
- Search customization beyond VitePress defaults
- 404 page customization (use VitePress default)
- Updating `outport.app` references to `outport.dev` across the codebase

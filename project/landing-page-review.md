# Landing Page & Positioning Review

**Date:** 2026-03-18
**Method:** Three parallel agents — OSS dev tool positioning research (10 successful tools), adversarial landing page critique (3 personas), messaging and differentiation analysis.

---

## OSS Dev Tool Landing Page Patterns (from studying 10 tools)

### What Works

1. **Problem-first, not feature-first.** mkcert describes the hell of manual certificates before mentioning itself. lazygit rants about git's UX. Portless shows ugly `localhost:3000` before the clean alternative.

2. **Before/after transformation.** Portless's diff format is the gold standard for CLI tools. Show the world before and after.

3. **One sentence that eliminates ambiguity:**
   - "A smarter cd command" (zoxide)
   - "Zero-config local HTTPS" (mkcert)
   - "A command runner, not a build system" (just)
   - "The front-end to your dev env" (mise)

4. **Animated terminal demo within 5 seconds of scrolling.** lazygit, zoxide, mkcert all show terminal recordings immediately.

5. **Two-command getting started.** `brew install X` then `X command`. If your tool requires system setup, separate that clearly from the basic "try it" path.

6. **"Replaces X, Y, Z" for consolidation tools.** mise explicitly names every tool it replaces.

7. **Saying what you are NOT.** just's "a command runner, not a build system" prevents the most common misunderstanding in six words.

8. **Honesty builds trust.** ripgrep has "Why shouldn't I use ripgrep?" mkcert has a security warning. Developers are allergic to marketing speak.

9. **Dual CTAs.** Primary install button + secondary docs/GitHub link.

10. **Tone matches category.** Minimal for utilities (direnv), playful for emotional tools (lazygit), professional for infrastructure (Caddy).

### What Doesn't Work

- Feature-first messaging without connecting to pain
- No visual proof (no terminal recordings/screenshots)
- Too many steps to try it
- Generic CTAs ("Get started" vs "Install in 30 seconds")
- Competitor bashing (none of the successful tools do it)
- Marketing speak ("revolutionary", "game-changing")
- Burying the install command

Source: [Evil Martians: "We studied 100 dev tool landing pages"](https://evilmartians.com/chronicles/we-studied-100-devtool-landing-pages-here-is-what-actually-works-in-2025)

---

## Landing Page Critique (Three Personas)

### Persona 1: Senior Backend Dev (3 Rails projects)

**Would install?** Maybe, but the page doesn't make the case.
- The headline describes a problem they have for 30 seconds every few months
- The three-step walkthrough shows them trading `PORT=3000` in .env for a YAML config + binary install + command — more work than the problem
- The monorepo example (buried in /guide/examples) is genuinely compelling but is three clicks deep
- **Need:** Show the pain of the third project — EADDRINUSE, wrong database, scattered .env

### Persona 2: AI/Worktree Power User

**Would install?** Would love it — but can't find their use case.
- Nothing on the homepage mentions worktrees, AI agents, or parallel development
- "Multi-instance" is jargon they don't connect to their workflow
- The "Work with AI" page exists but is about teaching agents to use Outport, not about WHY (solving parallel-agent port collisions)
- **Need:** A section showing main checkout vs worktree — different ports, different hostnames, zero effort

### Persona 3: Skeptical Senior Engineer

**Would install?** No, and would close the tab at "system start."
- Sees daemon + sudo + DNS + CA for port numbers — massively over-engineered
- No indication port allocation works without the daemon
- Getting started jumps to `outport system start` (sudo, daemon, CA) in step 2 — before showing any value
- **Need:** "Start with just `outport up` for deterministic ports. Add .test hostnames when you're ready."

---

## Headline Analysis

### Current: "Stop fighting port conflicts"

**Problems:**
- "Port conflicts" is an intermittent annoyance, not a chronic pain
- Reactive framing (stop a bad thing) vs proactive (gain a capability)
- Captures ~15% of Outport's value — doesn't hint at hostnames, computed values, multi-instance

### Three Alternatives

**Option A: Lead with the experience**
> `myapp.test` just works

Concrete, shows the after-state, invites curiosity ("how?"), visually demonstrable.

**Option B: Lead with the outcome**
> Stable dev environments, declared in one file

Appeals to infrastructure-as-code mindset. Slightly generic.

**Option C: Lead with the differentiator**
> Every instance gets its own ports, hostnames, and URLs

The thing nothing else does. Positions for AI/worktree growth. Requires reader to already feel the pain.

**Recommendation:** Option A headline + Option C as first supporting message.

### Subtitle

**Current:** "Port orchestration for multi-project development. Deterministic ports, .test hostnames with automatic HTTPS, .env integration, and multi-instance support — all from a single config file."

**Problem:** Feature list, not value proposition. "Port orchestration" means nothing to a cold visitor.

**Proposed:** "Declare your services in `.outport.yml`. Every checkout gets deterministic ports, `.test` hostnames with HTTPS, and fully wired `.env` files. Works across projects, worktrees, and parallel AI agents."

---

## Differentiation vs Portless

Three clearest differentiators (Portless has 4.7k stars, Vercel brand):

1. **Multi-service orchestration** — Portless handles one app at a time. Outport handles Rails + Postgres + Redis + Nuxt + Mailpit in a single config with cross-service computed values.

2. **`.env` as integration surface** — Portless requires a runtime wrapper (`portless -- npm run dev`). Outport writes to `.env` and gets out of the way. No wrapper, no SDK.

3. **Computed cross-service values** — `CORS_ORIGINS=${frontend.url},${portal.url}` — zero competition on this feature.

**Should the site mention competitors?** Not on the homepage. Create a `/guide/why-outport` page with the README comparison table.

---

## The "Aha Moment"

Different developers hit it at different moments:
- **Single-project:** "I can type `myapp.test` instead of `localhost:3000`? With HTTPS?"
- **Multi-project:** "All my projects get stable, non-colliding ports, configured once?"
- **Worktree/AI-agent:** "Each worktree gets its own ports AND hostname? Cookie isolation for free?"
- **Monorepo:** "My frontend .env automatically knows my backend's URL? And updates for tunnels?"

**Current demo doesn't create the aha.** It shows one instance of `outport up`. A two-frame demo showing main + worktree (same project, different ports and hostnames) would be much more powerful.

---

## Missing Content

### High Priority
1. **"Works without the daemon" messaging** — progressive adoption ladder (ports → hostnames → HTTPS → sharing)
2. **Two-frame multi-instance demo** on homepage
3. **AI agent / parallel development section** on homepage
4. **The problem narrative** — the README's "The Problem" section is better copy than the homepage

### Medium Priority
5. **Computed values on homepage** — `CORS_ORIGINS=${frontend.url}` is the zero-competition feature
6. **Why Outport page** with README comparison table
7. **Monorepo example** surfaced on homepage (currently buried in /guide/examples)

### Lower Priority
8. **Animated terminal demo** (asciinema or similar)
9. **macOS-only notice** on homepage (currently discovered at getting-started prerequisites)
10. **Warmer tone** — the site is understated to the point of being forgettable

---

## Structural Issues

- **Homepage shows the trivial case.** A single Rails app with Postgres and Redis is where Outport is HARDEST to justify. Monorepo and worktree scenarios are where it's OBVIOUSLY valuable — and they're buried.
- **Getting started scares skeptics.** `outport system start` (sudo + daemon + CA) is step 2, before user has seen any value.
- **Feature cards miss the differentiator.** Computed values / cross-service wiring has no card. Automatic HTTPS should be a sub-feature of .test Domains, not a standalone card.
- **Terminal demo duplicates "Get up and running" step 2.** Same output shown twice on one page.
- **Feature card icons (#, .t, >_, .e, S) are cryptic.** Don't communicate at a glance.

---

## What Works Well

- Visual design is clean and distinctive (warm cream palette, Barlow headings)
- Terminal mockups look professional
- The examples page (monorepo config) is excellent — strongest content on the site
- Getting started "What Just Happened" section builds trust
- Tips page is practical and specific
- Configuration reference is thorough
- `outport doctor` signals engineering maturity
- README comparison table is strong (but only in README, not on docs site)

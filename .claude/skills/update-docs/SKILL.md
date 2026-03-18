---
name: update-docs
description: Audit all documentation for staleness after code changes. Checks README, CLAUDE.md, init presets, and release docs. Use after completing a feature, merging a PR, or when asked to check docs.
---

# Update Docs

Run this after any meaningful change to the codebase. It checks every documentation location, determines what's stale, and updates it.

## Step 1 — Understand what changed

```bash
# Changes on the current branch vs master
git log --oneline master..HEAD
git diff --stat master..HEAD

# Or if working on master, recent commits
git log --oneline -10
```

Summarize the changes in one sentence. Classify:

| Type | Example |
|------|---------|
| **New command** | Added `outport open` |
| **Command change** | New flag, changed output format |
| **Config change** | New field in `.outport.yml` |
| **Infrastructure** | Build, CI, dependency changes |
| **Bug fix** | Fixed registry save race condition |

## Step 2 — Check each documentation location

Go through every location below. For each one, check whether it's current.

### README.md

| Check | What to look for |
|-------|-----------------|
| Commands list | Does it match actual commands in `cmd/*.go`? |
| Config example | Does `.outport.yml` example match current schema? |
| "How It Works" section | Does it reflect current allocation behavior? |
| Install instructions | Are they still accurate? |

### CLAUDE.md

| Check | What to look for |
|-------|-----------------|
| Architecture section | Do package descriptions match current code? |
| CLI commands list | Does it include all commands in `cmd/*.go`? |
| Key design decisions | Do they reflect current behavior? |
| Development commands | Does the justfile have new/changed recipes? |

### cmd/init.go presets

| Check | What to look for |
|-------|-----------------|
| Service presets | Does the `presets` slice include all standard services? |
| Preset fields | Do presets use the current config field names? |

### project/releasing.md

| Check | What to look for |
|-------|-----------------|
| Release process | Does it match current CI workflow? |
| Prerequisites | Are tools and secrets still correct? |

### .goreleaser.yml

| Check | What to look for |
|-------|-----------------|
| Build targets | Do platforms match what's documented? |
| Homebrew formula | Is the tap config current? |

### docs/ (VitePress site)

| Check | What to look for |
|-------|-----------------|
| `docs/reference/commands.md` | Does it list all commands in `cmd/*.go`? |
| `docs/reference/configuration.md` | Does it cover all `.outport.yml` fields? |
| `docs/guide/getting-started.md` | Does it reflect current workflow? |
| `docs/guide/examples.md` | Are examples using current config syntax? |
| `docs/guide/tips.md` | Are troubleshooting tips still accurate? |

### skills/outport/SKILL.md

| Check | What to look for |
|-------|-----------------|
| Quick Reference commands | Does it list all commands? |
| Common Tasks section | Does it cover current capabilities? |
| Trigger keywords | Do they cover all features? |

## Step 3 — Report findings

Present as a table, only showing items that need attention:

```
| Status | File | What needs updating |
|--------|------|---------------------|
| STALE  | README.md | Commands list missing `open` |
| OK     | CLAUDE.md | — |
```

## Step 4 — Make the updates

For each STALE item, update the file. Rules:

- **Don't duplicate content.** README is the user-facing overview. CLAUDE.md is the agent-facing architecture guide. Don't copy between them.
- **README** should describe what commands do, not how they're implemented.
- **CLAUDE.md** should describe architecture and conventions, not user-facing docs.

## Quick reference — what to update per change type

| Change type | Always update | Check if needed |
|-------------|--------------|-----------------|
| **New command** | README commands, CLAUDE.md commands, docs/reference/commands.md, skills/outport/SKILL.md | init presets, docs/guide/getting-started.md, docs/guide/tips.md |
| **Config schema change** | README config example, CLAUDE.md config description, docs/reference/configuration.md | init presets, docs/guide/examples.md |
| **New convention** | CLAUDE.md | — |
| **Infrastructure** | — | releasing.md, CLAUDE.md dev commands |
| **Bug fix** | — | README (if workaround documented), docs/guide/tips.md |

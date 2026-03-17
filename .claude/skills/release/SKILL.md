---
name: release
description: Release a new version of outport. Use when the user says "release", "let's release", "tag a release", "ship it", "cut a release", "push a new version", or asks about the release process. Also use when the user asks to deploy docs after a release.
---

# Release

Guides the release of a new outport version. Runs pre-flight checks, determines the version, tags, and deploys docs.

## Step 1 — Pre-flight

Verify master is clean and ready:

```bash
git checkout master
git pull
```

Check for uncommitted changes. If the working tree is dirty, stop and ask.

Run tests and lint:

```bash
just test
just lint
```

Both must pass before proceeding. If tests fail, stop and fix.

## Step 2 — Determine version

Check the latest tag:

```bash
git tag --sort=-v:refname | head -5
```

Look at what's changed since the last release:

```bash
git log --oneline $(git tag --sort=-v:refname | head -1)..HEAD
```

Suggest the next version based on what changed:
- **Patch** (v0.X.Y+1): bug fixes only
- **Minor** (v0.X+1.0): new features, non-breaking changes
- **Major**: breaking changes (rare pre-1.0 — minor bumps are fine for breaking changes while pre-1.0)

Present the suggestion with the commit list and ask the user to confirm the version number before proceeding.

## Step 3 — Tag and push

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

This triggers the GitHub Actions release workflow which builds binaries and updates the Homebrew tap.

## Step 4 — Deploy docs

Check if any docs changed since the last release:

```bash
git diff --name-only $(git tag --sort=-v:refname | head -2 | tail -1)..HEAD -- docs/ README.md
```

If docs changed, build and deploy:

```bash
npx vitepress build docs
npx wrangler pages deploy docs/.vitepress/dist --project-name outport-dev
```

If no docs changed, skip this step and say so.

## Step 5 — Verify

Check the GitHub Actions run:

```bash
gh run list --limit 3
```

If the latest run is still in progress, let the user know and suggest they check back. If it completed, report the status.

Tell the user to verify the install when ready:

```
brew update && brew upgrade outport && outport --version
```

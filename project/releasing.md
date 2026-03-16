# Releasing

## Prerequisites

- GoReleaser installed locally (`brew install goreleaser`)
- `HOMEBREW_TAP_TOKEN` secret configured on the GitHub repo (a GitHub PAT with repo scope for `steveclarke/homebrew-tap`)
- 1Password reference: `op://Employee/Github/5k3jgfymttcdl2bvase435ikji`

## Process

### 1. Prepare

Ensure master is clean and all changes are merged:

```bash
git checkout master
git pull
just test
just lint
```

### 2. Dry run

Verify GoReleaser will build correctly:

```bash
just release-dry-run
```

### 3. Tag and push

```bash
git tag v0.X.Y
git push origin v0.X.Y
```

This triggers the GitHub Actions release workflow (`.github/workflows/release.yml`), which:
1. Checks out the repo
2. Runs `go test ./...`
3. Runs GoReleaser, which:
   - Builds binaries for macOS + Linux (amd64 + arm64)
   - Creates a GitHub release with tar.gz archives and checksums
   - Updates the Homebrew formula in `steveclarke/homebrew-tap`

### 4. Deploy docs

If any documentation changes were included in this release, deploy the docs site:

```bash
npm run docs:build
npx wrangler pages deploy docs/.vitepress/dist --project-name outport-dev
```

Once [GitHub issue #37](https://github.com/steveclarke/outport/issues/37) is resolved, this step becomes automatic on push to master.

### 5. Verify

- Check the [GitHub Actions run](https://github.com/steveclarke/outport/actions) completed successfully
- Check the [GitHub release](https://github.com/steveclarke/outport/releases) has the correct assets
- Check that `steveclarke/homebrew-tap` has a new commit updating the formula

### 6. Test the install

```bash
brew update
brew upgrade outport
outport --version
```

## Version scheme

Follow semver. The GoReleaser changelog auto-excludes `docs:`, `chore:`, and `test:` commits.

## Troubleshooting

**GoReleaser fails on GitHub Actions:**
- Check that `HOMEBREW_TAP_TOKEN` secret is set and the PAT hasn't expired
- Check that the tag matches the `v*` pattern

**Homebrew formula not updated:**
- The PAT needs `repo` scope for `steveclarke/homebrew-tap`
- Verify the token hasn't expired in 1Password

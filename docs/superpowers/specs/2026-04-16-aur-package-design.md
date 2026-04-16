# AUR Package for Arch Linux

**Issue:** #106
**Date:** 2026-04-16

## Summary

Add an `outport-bin` package to the Arch User Repository (AUR) and automate publishing via a dedicated GitHub Actions workflow triggered on each release.

## PKGBUILD

The PKGBUILD template lives at `packaging/aur/PKGBUILD.template` with placeholder variables substituted at publish time.

**Package metadata:**

- **Name:** `outport-bin` (AUR convention for pre-built binary packages)
- **Architecture:** `x86_64`, `aarch64`
- **Source:** architecture-specific tarballs from GitHub Releases (`source_x86_64`, `source_aarch64`)
- **Checksums:** SHA-256, extracted from the release's `checksums.txt`
- **depends:** none (static Go binary, `CGO_ENABLED=0`)
- **optdepends:** `cloudflared` (tunnel support for `outport share`)
- **provides/conflicts:** `('outport')` — conflicts with a hypothetical `outport` source package

**Install paths (Arch conventions):**

| File | Destination |
|------|-------------|
| Binary | `/usr/bin/outport` |
| Bash completion | `/usr/share/bash-completion/completions/outport` |
| Zsh completion | `/usr/share/zsh/site-functions/_outport` |
| Fish completion | `/usr/share/fish/vendor_completions.d/outport.fish` |
| License | `/usr/share/licenses/outport-bin/LICENSE` |

**Template placeholders:** `{{VERSION}}`, `{{SHA256_AMD64}}`, `{{SHA256_ARM64}}`

## CI Workflow

A new workflow file `.github/workflows/aur-publish.yml`, separate from the existing release workflow.

**Trigger:** `release: published` — runs automatically after GoReleaser completes and the GitHub Release is published.

**Environment:** Arch Linux container (`archlinux:base-devel`)

**Steps:**

1. Extract version from the release tag (strip `v` prefix — e.g., `v0.42.0` → `0.42.0`)
2. Fetch `checksums.txt` from the release assets
3. Extract SHA-256 sums for `outport_VERSION_linux_amd64.tar.gz` and `outport_VERSION_linux_arm64.tar.gz`
4. Generate PKGBUILD from template via `sed`/`envsubst` — substitute version and checksums
5. Generate `.SRCINFO` via `makepkg --printsrcinfo`
6. Clone AUR repo via SSH, copy in PKGBUILD and .SRCINFO, commit, push

**Secrets:** `AUR_KEY` — contents of the ed25519 private key for the `sclarke77` AUR account.

**Properties:** Idempotent (re-running for the same release pushes the same PKGBUILD). Can be manually re-triggered from the GitHub Actions UI if the AUR push fails. AUR failures do not affect the main release, Homebrew tap, or GitHub Release.

## File Layout

```
packaging/aur/
├── PKGBUILD.template    # Template with placeholders
└── README.md            # How the template is used (contributor context)

.github/workflows/
├── release.yml          # Existing — unchanged
└── aur-publish.yml      # New — triggered by release:published
```

The AUR git repo (`aur@aur.archlinux.org/outport-bin.git`) contains only `PKGBUILD` and `.SRCINFO` — generated files, never committed to this repo.

Private setup docs (AUR account details, key location, manual testing steps) go in the backstage repo.

## Initial Bootstrap (One-Time)

Before CI can publish, the AUR package must be created manually:

1. Clone the empty AUR repo: `git clone ssh://aur@aur.archlinux.org/outport-bin.git`
2. Generate the PKGBUILD from the template for the current release
3. Test locally with `makepkg -si` on the Arch VM
4. Generate `.SRCINFO` via `makepkg --printsrcinfo`
5. Commit and push — this creates the package listing on AUR
6. Add the `AUR_KEY` secret to the GitHub repo settings
7. Delete the local clone — CI handles all future releases

## Out of Scope

- GoReleaser AUR publisher (requires Pro license)
- Source-build `outport` package (would require Go toolchain as build dep)
- AUR package for non-Linux platforms (AUR is Arch-only)

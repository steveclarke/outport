# AUR Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `outport-bin` to the AUR and automate updates on every GitHub Release.

**Architecture:** A PKGBUILD template in `packaging/aur/` gets version/checksum placeholders substituted by a GitHub Actions workflow. The workflow runs in an Arch container, generates `.SRCINFO`, and pushes to the AUR git repo via SSH. Completely decoupled from the existing release pipeline.

**Tech Stack:** PKGBUILD (bash), GitHub Actions, Arch Linux container, SSH

**Spec:** `docs/superpowers/specs/2026-04-16-aur-package-design.md`

---

### Task 1: Create the PKGBUILD template

**Files:**
- Create: `packaging/aur/PKGBUILD.template`

- [ ] **Step 1: Create the template file**

```bash
# Maintainer: Steve Clarke <steve@outport.dev>
pkgname=outport-bin
pkgver={{VERSION}}
pkgrel=1
pkgdesc="Dev port manager for multi-project, multi-worktree development"
arch=('x86_64' 'aarch64')
url="https://outport.dev"
license=('MIT')
depends=()
optdepends=('cloudflared: tunnel support for outport share')
provides=('outport')
conflicts=('outport')
source_x86_64=("https://github.com/steveclarke/outport/releases/download/v${pkgver}/outport_${pkgver}_linux_amd64.tar.gz")
source_aarch64=("https://github.com/steveclarke/outport/releases/download/v${pkgver}/outport_${pkgver}_linux_arm64.tar.gz")
sha256sums_x86_64=('{{SHA256_AMD64}}')
sha256sums_aarch64=('{{SHA256_ARM64}}')

package() {
    install -Dm755 outport "${pkgdir}/usr/bin/outport"
    install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
    install -Dm644 completions/outport.bash "${pkgdir}/usr/share/bash-completion/completions/outport"
    install -Dm644 completions/_outport "${pkgdir}/usr/share/zsh/site-functions/_outport"
    install -Dm644 completions/outport.fish "${pkgdir}/usr/share/fish/vendor_completions.d/outport.fish"
}
```

- [ ] **Step 2: Verify the template references match the actual tar.gz contents**

The GoReleaser archive contains these files at the root level (no subdirectory):
```
outport
LICENSE
completions/outport.bash
completions/_outport
completions/outport.fish
```

Confirm the `package()` paths match. They do — `install -Dm755 outport` references the binary at root, and `completions/` paths match the archive structure.

- [ ] **Step 3: Commit**

```bash
git add packaging/aur/PKGBUILD.template
git commit -m "feat: add AUR PKGBUILD template for outport-bin"
```

---

### Task 2: Create the packaging README

**Files:**
- Create: `packaging/aur/README.md`

- [ ] **Step 1: Create the README**

```markdown
# AUR Package: outport-bin

This directory contains the PKGBUILD template for the `outport-bin` AUR package.

## How it works

On each GitHub Release, the `aur-publish.yml` workflow:

1. Reads `checksums.txt` from the release assets
2. Substitutes `{{VERSION}}`, `{{SHA256_AMD64}}`, and `{{SHA256_ARM64}}` in `PKGBUILD.template`
3. Generates `.SRCINFO` via `makepkg --printsrcinfo`
4. Pushes to `aur@aur.archlinux.org/outport-bin.git`

## Manual testing

On an Arch system, generate a PKGBUILD from the template and test it:

```bash
VERSION=0.41.0
    SHA256_AMD64=$(curl -sL https://github.com/steveclarke/outport/releases/download/v${VERSION}/checksums.txt | grep linux_amd64.tar.gz | awk '{print $1}')
    SHA256_ARM64=$(curl -sL https://github.com/steveclarke/outport/releases/download/v${VERSION}/checksums.txt | grep linux_arm64.tar.gz | awk '{print $1}')
    sed -e "s/{{VERSION}}/${VERSION}/" -e "s/{{SHA256_AMD64}}/${SHA256_AMD64}/" -e "s/{{SHA256_ARM64}}/${SHA256_ARM64}/" PKGBUILD.template > PKGBUILD
    makepkg -si
    outport --version
```

- [ ] **Step 2: Commit**

```bash
git add packaging/aur/README.md
git commit -m "docs: add AUR packaging README"
```

---

### Task 3: Create the AUR publish workflow

**Files:**
- Create: `.github/workflows/aur-publish.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: Publish AUR Package

on:
  release:
    types: [published]

jobs:
  aur-publish:
    runs-on: ubuntu-latest
    container:
      image: archlinux:base-devel
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Install dependencies
        run: pacman -Sy --noconfirm git openssh

      - name: Extract version
        id: version
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          echo "version=${VERSION}" >> "$GITHUB_OUTPUT"

      - name: Download checksums
        run: |
          curl -sL "https://github.com/steveclarke/outport/releases/download/${GITHUB_REF_NAME}/checksums.txt" -o checksums.txt

      - name: Extract SHA256 sums
        id: checksums
        run: |
          VERSION="${{ steps.version.outputs.version }}"
          SHA256_AMD64=$(grep "outport_${VERSION}_linux_amd64.tar.gz" checksums.txt | awk '{print $1}')
          SHA256_ARM64=$(grep "outport_${VERSION}_linux_arm64.tar.gz" checksums.txt | awk '{print $1}')
          echo "sha256_amd64=${SHA256_AMD64}" >> "$GITHUB_OUTPUT"
          echo "sha256_arm64=${SHA256_ARM64}" >> "$GITHUB_OUTPUT"

      - name: Generate PKGBUILD
        run: |
          sed \
            -e "s/{{VERSION}}/${{ steps.version.outputs.version }}/" \
            -e "s/{{SHA256_AMD64}}/${{ steps.checksums.outputs.sha256_amd64 }}/" \
            -e "s/{{SHA256_ARM64}}/${{ steps.checksums.outputs.sha256_arm64 }}/" \
            packaging/aur/PKGBUILD.template > PKGBUILD

      - name: Generate .SRCINFO
        run: |
          useradd -m builder
          chown builder:builder PKGBUILD
          su builder -c "makepkg --printsrcinfo" > .SRCINFO

      - name: Push to AUR
        env:
          AUR_KEY: ${{ secrets.AUR_KEY }}
        run: |
          mkdir -p ~/.ssh
          echo "$AUR_KEY" > ~/.ssh/aur
          chmod 600 ~/.ssh/aur
          cat >> ~/.ssh/config <<EOF
          Host aur.archlinux.org
            IdentityFile ~/.ssh/aur
            StrictHostKeyChecking accept-new
          EOF

          git clone ssh://aur@aur.archlinux.org/outport-bin.git aur-repo
          cp PKGBUILD .SRCINFO aur-repo/
          cd aur-repo
          git config user.name "Steve Clarke"
          git config user.email "steve@outport.dev"
          git add PKGBUILD .SRCINFO
          git diff --cached --quiet && echo "No changes to push" && exit 0
          git commit -m "Update to v${{ steps.version.outputs.version }}"
          git push
```

- [ ] **Step 2: Review the workflow step by step**

Verify:
- `release: types: [published]` triggers after the main release workflow completes
- `archlinux:base-devel` provides `makepkg` out of the box
- `useradd builder` is needed because `makepkg` refuses to run as root
- `git diff --cached --quiet` makes it idempotent — no empty commits on re-runs
- SSH config uses `accept-new` for the first connection (AUR host key not pre-known)
- The key is written from the secret, used, and discarded with the container

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/aur-publish.yml
git commit -m "feat: add AUR publish workflow triggered on release"
```

---

### Task 4: Test the PKGBUILD on the Arch VM

This task is manual — run it on the Arch Linux VM at 192.168.77.16.

- [ ] **Step 1: Copy the template to the VM**

```bash
scp packaging/aur/PKGBUILD.template 192.168.77.16:~/outport-aur/
```

- [ ] **Step 2: SSH in and generate the PKGBUILD for v0.41.0**

```bash
ssh 192.168.77.16
mkdir -p ~/outport-aur && cd ~/outport-aur

VERSION=0.41.0
SHA256_AMD64=$(curl -sL https://github.com/steveclarke/outport/releases/download/v${VERSION}/checksums.txt | grep linux_amd64.tar.gz | awk '{print $1}')
SHA256_ARM64=$(curl -sL https://github.com/steveclarke/outport/releases/download/v${VERSION}/checksums.txt | grep linux_arm64.tar.gz | awk '{print $1}')

sed \
  -e "s/{{VERSION}}/${VERSION}/" \
  -e "s/{{SHA256_AMD64}}/${SHA256_AMD64}/" \
  -e "s/{{SHA256_ARM64}}/${SHA256_ARM64}/" \
  PKGBUILD.template > PKGBUILD
```

- [ ] **Step 3: Build and install the package**

```bash
makepkg -si
```

Expected: downloads the tarball, verifies checksum, installs to `/usr/bin/outport`.

- [ ] **Step 4: Verify the installation**

```bash
outport --version
# Expected: outport version 0.41.0

which outport
# Expected: /usr/bin/outport

outport completion bash > /dev/null
# Expected: no error (completions were installed)

pacman -Ql outport-bin
# Expected: lists all installed files matching the PKGBUILD install paths
```

- [ ] **Step 5: Clean up**

```bash
sudo pacman -R outport-bin
```

---

### Task 5: Bootstrap the AUR package (one-time)

This task is manual — run after verifying the PKGBUILD works in Task 4.

- [ ] **Step 1: Clone the empty AUR repo**

```bash
git clone ssh://aur@aur.archlinux.org/outport-bin.git ~/outport-aur-repo
cd ~/outport-aur-repo
```

This can be done from any machine with the AUR SSH key configured (Mac or VM).

- [ ] **Step 2: Generate and add the PKGBUILD**

```bash
# Copy the generated PKGBUILD from Task 4 (or regenerate from template)
cp ~/outport-aur/PKGBUILD .
makepkg --printsrcinfo > .SRCINFO
```

Note: `makepkg --printsrcinfo` must run on Arch. If bootstrapping from macOS, generate `.SRCINFO` on the VM and copy it over.

- [ ] **Step 3: Commit and push to AUR**

```bash
git add PKGBUILD .SRCINFO
git commit -m "Initial release: outport-bin 0.41.0"
git push
```

This creates the package at https://aur.archlinux.org/packages/outport-bin

- [ ] **Step 4: Verify on the AUR website**

Open https://aur.archlinux.org/packages/outport-bin and confirm the package page exists with correct metadata.

- [ ] **Step 5: Add the AUR_KEY secret to GitHub**

Go to https://github.com/steveclarke/outport/settings/secrets/actions and add:
- **Name:** `AUR_KEY`
- **Value:** contents of `~/.ssh/aur` (the ed25519 private key)

- [ ] **Step 6: Clean up local clone**

```bash
rm -rf ~/outport-aur-repo ~/outport-aur
```

---

### Task 6: Document the setup in backstage

**Files:**
- Create or update: `~/src/backstage/outport/aur-package.md`

- [ ] **Step 1: Write the private setup docs**

```markdown
# AUR Package: outport-bin

## AUR Account
- **Username:** sclarke77
- **SSH key:** `~/.ssh/aur` (ed25519, configured in `~/.ssh/config`)
- **Package URL:** https://aur.archlinux.org/packages/outport-bin

## GitHub Secret
- **Name:** `AUR_KEY`
- **Contains:** contents of `~/.ssh/aur` private key

## How releases work
1. Push a `v*` tag
2. `release.yml` runs GoReleaser → GitHub Release with tarballs + checksums
3. `aur-publish.yml` triggers on `release: published` → generates PKGBUILD → pushes to AUR

## Manual re-publish
If the AUR publish fails, re-run the workflow from the GitHub Actions UI:
https://github.com/steveclarke/outport/actions/workflows/aur-publish.yml

## Manual PKGBUILD generation
See `packaging/aur/README.md` in the outport repo for instructions.
```

- [ ] **Step 2: Commit in backstage**

```bash
cd ~/src/backstage
git add outport/aur-package.md
git commit -m "docs: add AUR package setup notes"
git push
```

---

### Task 7: Update outport docs and README

**Files:**
- Modify: `README.md` (add AUR install method)
- Modify: `docs/` (if there's an installation page, add AUR)

- [ ] **Step 1: Add AUR to the README install section**

Add after the existing Homebrew/deb/rpm install methods:

```markdown
### Arch Linux (AUR)

```bash
# With an AUR helper (e.g., yay, paru)
yay -S outport-bin

# Or manually
git clone https://aur.archlinux.org/outport-bin.git
cd outport-bin
makepkg -si
```

- [ ] **Step 2: Update the docs site install page if one exists**

Check `docs/` for an installation guide and add the same AUR instructions.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/
git commit -m "docs: add AUR install instructions"
```

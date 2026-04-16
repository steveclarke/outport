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

---
description: Install Outport via the install script, Homebrew, .deb/.rpm package, go install, or build from source. macOS and Linux supported.
---

# Installation

## Install Script (Recommended)

Works on macOS and Linux. Downloads the latest release, verifies the checksum, and installs to `~/.local/bin`:

```bash
curl -fsSL https://outport.dev/install.sh | sh
```

The binary is installed to `~/.local/bin/outport` (or `/usr/local/bin/outport` when run as root). If the install directory is not in your `PATH`, the script will show the exact command to add it.

Options:

```bash
# Install to a specific directory
curl -fsSL https://outport.dev/install.sh | sh -s -- --dir /usr/local/bin

# Install a specific version
curl -fsSL https://outport.dev/install.sh | sh -s -- --version 0.30.0
```

## Homebrew

```bash
brew install steveclarke/tap/outport
```

To update:

```bash
brew upgrade outport
```

## .deb / .rpm Package

Download the `.deb` or `.rpm` from the [latest release](https://github.com/steveclarke/outport/releases/latest):

```bash
# Debian / Ubuntu
sudo dpkg -i outport_*.deb

# Fedora / RHEL
sudo rpm -i outport-*.rpm
```

## From Source

Requires [Go 1.26+](https://go.dev/dl/):

```bash
go install github.com/steveclarke/outport@latest
```

This installs to `$GOPATH/bin`. Make sure it's in your `PATH`.

## Build Locally

```bash
git clone https://github.com/steveclarke/outport.git
cd outport
go build -o outport .
```

Or using [just](https://github.com/casey/just):

```bash
just build      # Compiles to dist/outport
just install    # Installs to $GOPATH/bin
```

## Shell Completions

Outport supports tab completion for bash, zsh, and fish. Homebrew and .deb/.rpm packages install completions automatically. For other install methods:

```bash
# Bash — add to ~/.bashrc
eval "$(outport completion bash)"

# Zsh — add to ~/.zshrc (after compinit)
eval "$(outport completion zsh)"

# Fish — run once
outport completion fish > ~/.config/fish/completions/outport.fish
```

## Verify

```bash
outport --version
```

---
description: Install Outport via Homebrew, go install, or build from source. macOS and Linux supported.
---

# Installation

## Homebrew (Recommended)

```bash
brew install steveclarke/tap/outport
```

To update:

```bash
brew upgrade outport
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

## Verify

```bash
outport --version
```

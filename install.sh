#!/bin/sh
# Outport installer — https://outport.dev
#
# Usage:
#   curl -fsSL https://outport.dev/install.sh | sh
#   curl -fsSL https://outport.dev/install.sh | sh -s -- --dir /usr/local/bin
#
# Options:
#   --dir DIR    Install to DIR (default: ~/.local/bin, or /usr/local/bin with sudo)
#   --version V  Install a specific version (default: latest)

set -eu

REPO="steveclarke/outport"
BINARY="outport"

# --- Helpers ---------------------------------------------------------------

# Colors — disabled when piped or when terminal doesn't support them
if [ -t 1 ] && [ "${TERM:-}" != "dumb" ]; then
  GREEN=$(printf '\033[32m')
  YELLOW=$(printf '\033[33m')
  RESET=$(printf '\033[0m')
else
  GREEN=""
  YELLOW=""
  RESET=""
fi

say() {
  printf '  %s\n' "$@"
}

err() {
  printf 'Error: %s\n' "$@" >&2
  exit 1
}

need() {
  if ! command -v "$1" > /dev/null 2>&1; then
    err "$1 is required but not found"
  fi
}

# --- Detect OS and architecture --------------------------------------------

detect_os() {
  os=$(uname -s)
  case "$os" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux" ;;
    *)      err "Unsupported OS: $os" ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)             err "Unsupported architecture: $arch" ;;
  esac
}

# --- Resolve install directory ---------------------------------------------

default_install_dir() {
  if [ "$(id -u)" = "0" ]; then
    echo "/usr/local/bin"
  else
    echo "$HOME/.local/bin"
  fi
}

ensure_dir_in_path() {
  dir="$1"
  case ":$PATH:" in
    *":$dir:"*) return 0 ;;
  esac

  shell_name=$(basename "${SHELL:-/bin/sh}")
  case "$shell_name" in
    bash) rc="$HOME/.bashrc" ;;
    zsh)  rc="$HOME/.zshrc" ;;
    fish) rc="$HOME/.config/fish/config.fish" ;;
    *)    rc="" ;;
  esac

  echo ""
  say "${YELLOW}$dir is not in your PATH.${RESET}"
  if [ -n "$rc" ]; then
    say "Add it by running:"
    echo ""
    if [ "$shell_name" = "fish" ]; then
      say "  fish_add_path $dir"
    else
      say "  echo 'export PATH=\"$dir:\$PATH\"' >> $rc"
    fi
    echo ""
    say "Then restart your shell or run: source $rc"
  else
    say "Add $dir to your PATH."
  fi
}

# --- Fetch latest version --------------------------------------------------

latest_version() {
  need curl
  url="https://api.github.com/repos/$REPO/releases/latest"
  version=$(curl -fsSL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/')
  if [ -z "$version" ]; then
    err "Could not determine latest version"
  fi
  echo "$version"
}

# --- Download and verify ---------------------------------------------------

download_and_install() {
  version="$1"
  os="$2"
  arch="$3"
  install_dir="$4"

  archive="${BINARY}_${version}_${os}_${arch}.tar.gz"
  url="https://github.com/$REPO/releases/download/v${version}/${archive}"
  checksum_url="https://github.com/$REPO/releases/download/v${version}/checksums.txt"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  say "Downloading $BINARY v$version ($os/$arch)..."
  curl -fsSL "$url" -o "$tmpdir/$archive" || err "Download failed — version v$version may not exist"
  curl -fsSL "$checksum_url" -o "$tmpdir/checksums.txt" || err "Checksum download failed"

  # Verify checksum
  expected=$(grep "$archive" "$tmpdir/checksums.txt" | awk '{print $1}')
  if [ -z "$expected" ]; then
    err "No checksum found for $archive"
  fi

  if command -v sha256sum > /dev/null 2>&1; then
    actual=$(sha256sum "$tmpdir/$archive" | awk '{print $1}')
  elif command -v shasum > /dev/null 2>&1; then
    actual=$(shasum -a 256 "$tmpdir/$archive" | awk '{print $1}')
  else
    say "Warning: no sha256sum or shasum found, skipping checksum verification"
    actual="$expected"
  fi

  if [ "$expected" != "$actual" ]; then
    err "Checksum mismatch: expected $expected, got $actual"
  fi

  # Extract and install
  tar -xzf "$tmpdir/$archive" -C "$tmpdir"

  mkdir -p "$install_dir"
  mv "$tmpdir/$BINARY" "$install_dir/$BINARY"
  chmod +x "$install_dir/$BINARY"
}

# --- Main ------------------------------------------------------------------

main() {
  install_dir=""
  version=""

  while [ $# -gt 0 ]; do
    case "$1" in
      --dir)     install_dir="$2"; shift 2 ;;
      --version) version="$2"; shift 2 ;;
      *)         err "Unknown option: $1" ;;
    esac
  done

  os=$(detect_os)
  arch=$(detect_arch)

  if [ -z "$install_dir" ]; then
    install_dir=$(default_install_dir)
  fi

  if [ -z "$version" ]; then
    say "Fetching latest version..."
    version=$(latest_version)
  fi

  download_and_install "$version" "$os" "$arch" "$install_dir"

  echo ""
  say "${GREEN}Outport v$version installed to $install_dir/$BINARY${RESET}"

  installed_version=$("$install_dir/$BINARY" --version 2>/dev/null || true)
  if [ -n "$installed_version" ]; then
    say "Version: $installed_version"
  fi

  ensure_dir_in_path "$install_dir"

  echo ""
  say "Get started:"
  say "  $BINARY setup"
  say "  $BINARY init"
  echo ""
  say "Docs: https://outport.dev"
}

main "$@"

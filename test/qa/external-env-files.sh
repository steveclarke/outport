#!/bin/bash
#
# Manual QA tests for external env file safety.
#
# Usage:
#   ./test/qa/external-env-files.sh setup    # Create test projects
#   ./test/qa/external-env-files.sh teardown  # Remove test projects
#
# After setup, cd into each project and run the scenarios described below.
#
# Test projects:
#   test/qa/workdir/internal/     — Basic internal env files (no prompts)
#   test/qa/workdir/external/     — External path (../target/.env)
#   test/qa/workdir/target/       — Target directory for external writes
#   test/qa/workdir/symlink/      — Symlink inside project pointing outside
#
# Scenarios:
#
#   1. Internal paths (cd test/qa/workdir/internal)
#      outport up          → should work, no prompts
#      outport down        → should clean up
#
#   2. External path denied (cd test/qa/workdir/external)
#      outport up          → should error with usage (non-interactive)
#
#   3. External path approved (cd test/qa/workdir/external)
#      outport up -y       → should write + yellow warning at bottom
#      cat ../target/.env  → should contain PORT=
#      outport up -y --json | jq .external_files  → should show external file
#
#   4. Approval persists (cd test/qa/workdir/external)
#      outport up          → should succeed (no -y needed, approval remembered)
#
#   5. Force resets approval (cd test/qa/workdir/external)
#      outport up --force  → should error (approval cleared)
#      outport up --force -y → should succeed
#
#   6. Interactive prompt (cd test/qa/workdir/external)
#      outport down -y     → clean up first
#      outport up          → should show prompt, type "n" → aborts silently
#      outport up          → should show prompt, type "yes" → approves
#
#   7. Down cleans external (cd test/qa/workdir/external)
#      outport up -y       → set up first
#      outport down -y     → should clean external .env
#      cat ../target/.env  → PORT should be gone
#
#   8. Symlink detection (cd test/qa/workdir/symlink)
#      outport up          → should detect symlink escapes, require approval
#      outport up -y       → should write through symlink + warn
#      outport down -y     → clean up

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORKDIR="$SCRIPT_DIR/workdir"

setup() {
    echo "Setting up QA test projects in $WORKDIR..."

    rm -rf "$WORKDIR"
    mkdir -p "$WORKDIR"/{internal,external,target,symlink}

    # Symlink target (outside symlink project)
    mkdir -p /tmp/outport-qa-symlink-target

    # internal — basic project, no external files
    cd "$WORKDIR/internal"
    git init -q
    cat > .outport.yml << 'YAML'
name: qa-internal
services:
  web:
    preferred_port: 3000
    env_var: PORT
YAML

    # external — writes to ../target/.env
    cd "$WORKDIR/external"
    git init -q
    cat > .outport.yml << 'YAML'
name: qa-external
services:
  web:
    preferred_port: 3000
    env_var: PORT
    env_file: ../target/.env
YAML

    # symlink — symlink inside project pointing outside
    cd "$WORKDIR/symlink"
    git init -q
    ln -sf /tmp/outport-qa-symlink-target linked
    cat > .outport.yml << 'YAML'
name: qa-symlink
services:
  web:
    preferred_port: 3000
    env_var: PORT
    env_file: linked/.env
YAML

    echo ""
    echo "Ready. Test projects:"
    echo "  cd $WORKDIR/internal   # basic, no prompts"
    echo "  cd $WORKDIR/external   # external path, requires approval"
    echo "  cd $WORKDIR/symlink    # symlink trick, requires approval"
    echo ""
    echo "Run: ./test/qa/external-env-files.sh teardown  to clean up"
}

teardown() {
    echo "Cleaning up QA test projects..."

    # Run outport down in each project that might be registered
    for dir in "$WORKDIR"/internal "$WORKDIR"/external "$WORKDIR"/symlink; do
        if [ -d "$dir" ] && [ -f "$dir/.outport.yml" ]; then
            (cd "$dir" && outport down -y 2>/dev/null || true)
        fi
    done

    rm -rf "$WORKDIR"
    rm -rf /tmp/outport-qa-symlink-target

    echo "Done."
}

case "${1:-}" in
    setup)    setup ;;
    teardown) teardown ;;
    *)
        echo "Usage: $0 {setup|teardown}"
        exit 1
        ;;
esac

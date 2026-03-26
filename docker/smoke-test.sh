#!/usr/bin/env bash
#
# Smoke test for Outport inside the Linux dev container.
# Run: just dev-linux-shell, then: bash docker/smoke-test.sh
#
set -euo pipefail

PASS="\033[32mPASS\033[0m"
FAIL="\033[31mFAIL\033[0m"
failures=0

check() {
    local name="$1"
    shift
    if "$@" > /dev/null 2>&1; then
        echo -e "  $PASS  $name"
    else
        echo -e "  $FAIL  $name"
        failures=$((failures + 1))
    fi
}

echo "=== Building outport ==="
cd /src
just build
cp dist/outport /usr/local/bin/outport
echo ""

echo "=== Smoke tests ==="
check "outport --version" outport --version
check "outport --help" outport --help

echo ""
echo "=== Port allocation test ==="
cd /src/docker/example-app

# Clean state
outport down 2>/dev/null || true
rm -f .env

outport up
check "outport up succeeded" test -f .env
check ".env contains PORT" grep -q '^PORT=' .env

echo ""
echo "=== Example app test ==="
# Extract the assigned port
PORT_VAL=$(grep -E '^PORT=' .env | cut -d= -f2)
echo "  Assigned port: $PORT_VAL"

# Start example app in background
PORT="$PORT_VAL" go run main.go &
APP_PID=$!

# Wait for app to be ready (retry instead of fixed sleep)
app_ready=false
for i in {1..10}; do
    if curl -sf "http://127.0.0.1:${PORT_VAL}/" > /dev/null 2>&1; then
        app_ready=true
        break
    fi
    sleep 0.2
done
check "app responds on allocated port" test "$app_ready" = "true"

# Clean up app
kill "$APP_PID" 2>/dev/null || true
wait "$APP_PID" 2>/dev/null || true

echo ""
echo "=== outport status ==="
outport status

echo ""
echo "=== Cleanup ==="
outport down
rm -f .env
check "env file removed after down" test ! -f .env

echo ""
echo "=== systemd status ==="
check "systemd is running" bash -c "systemctl is-system-running --quiet || systemctl is-system-running"

# systemd-resolved may not start in Docker's stripped-down environment.
# Report its status but don't fail the smoke test — Phase 2 will address this.
resolved_status=$(systemctl is-active systemd-resolved 2>/dev/null || true)
echo "  INFO  systemd-resolved: $resolved_status"

echo ""
echo "=== Platform module test ==="
# Verify the Linux platform functions compile and the service unit can be generated
outport_bin=$(which outport)
check "outport binary found" test -n "$outport_bin"

# Test that outport system start detects Linux (not "unsupported")
# It will fail (no sudo in container) but should NOT say "only supported on macOS"
start_output=$(outport system start 2>&1 || true)
if echo "$start_output" | grep -q "only supported on macOS"; then
    echo -e "  $FAIL  platform detection (still shows macOS-only error)"
    failures=$((failures + 1))
else
    echo -e "  $PASS  platform detection (recognizes Linux)"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo -e "\033[31m$failures test(s) failed\033[0m"
    exit 1
else
    echo -e "\033[32mAll smoke tests passed\033[0m"
fi

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
        ((failures++))
    fi
}

echo "=== Building outport ==="
cd /src
go build -o /usr/local/bin/outport .
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
sleep 1

check "app responds on allocated port" curl -sf "http://127.0.0.1:${PORT_VAL}/"

# Clean up app
kill "$APP_PID" 2>/dev/null || true
wait "$APP_PID" 2>/dev/null || true

echo ""
echo "=== outport ports ==="
outport ports

echo ""
echo "=== Cleanup ==="
outport down
rm -f .env
check "env file removed after down" test ! -f .env

echo ""
echo "=== systemd status ==="
check "systemd is running" systemctl is-system-running --quiet || systemctl is-system-running 2>/dev/null
check "systemd-resolved is available" systemctl is-active systemd-resolved

echo ""
if [ "$failures" -gt 0 ]; then
    echo -e "\033[31m$failures test(s) failed\033[0m"
    exit 1
else
    echo -e "\033[32mAll smoke tests passed\033[0m"
fi

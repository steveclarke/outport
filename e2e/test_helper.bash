#!/usr/bin/env bash
# Shared setup, teardown, and helpers for Outport BATS E2E tests.

# Load bats assertion libraries.
# On macOS (Homebrew): /opt/homebrew/lib/bats-*
# On Linux (npm/git):  check BATS_SUPPORT_HOME or fall back to /usr/lib/bats-*
_load_lib() {
  local name="$1"
  if [[ -d "/opt/homebrew/lib/${name}" ]]; then
    load "/opt/homebrew/lib/${name}/load.bash"
  elif [[ -d "/usr/lib/${name}" ]]; then
    load "/usr/lib/${name}/load.bash"
  elif [[ -n "${BATS_LIB_PATH:-}" ]]; then
    # npm-installed or CI custom path
    load "${BATS_LIB_PATH}/${name}/load.bash"
  else
    echo "ERROR: Cannot find ${name}. Install via Homebrew or set BATS_LIB_PATH." >&2
    return 1
  fi
}

_load_lib bats-support
_load_lib bats-assert

# -------------------------------------------------------------------
# Binary path — always use an absolute path to the built binary.
# -------------------------------------------------------------------
OUTPORT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)/dist/outport"
export OUTPORT

# -------------------------------------------------------------------
# setup / teardown — every test gets an isolated temp directory
# with HOME set to it and a fake .git dir for config detection.
# -------------------------------------------------------------------
setup() {
  ORIG_HOME="$HOME"
  TEST_TMPDIR="$(mktemp -d)"
  export HOME="$TEST_TMPDIR"
  cd "$TEST_TMPDIR" || return 1
  mkdir .git
}

teardown() {
  export HOME="$ORIG_HOME"
  rm -rf "$TEST_TMPDIR"
}

# -------------------------------------------------------------------
# Helpers
# -------------------------------------------------------------------

# create_config writes the given YAML string to outport.yml in the
# current directory.
create_config() {
  cat > outport.yml <<< "$1"
}

# assert_json_ok verifies that the output contains a JSON envelope
# with "ok": true.
assert_json_ok() {
  local ok
  ok="$(echo "$output" | jq -r '.ok')"
  [[ "$ok" == "true" ]] || {
    echo "Expected .ok to be true, got: $ok" >&2
    echo "Full output: $output" >&2
    return 1
  }
}

# assert_json_error verifies that the output contains a JSON envelope
# with "ok": false.
assert_json_error() {
  local ok
  ok="$(echo "$output" | jq -r '.ok')"
  [[ "$ok" == "false" ]] || {
    echo "Expected .ok to be false, got: $ok" >&2
    echo "Full output: $output" >&2
    return 1
  }
}

# json_data extracts the .data field from the JSON envelope.
json_data() {
  echo "$output" | jq '.data'
}

# json_field extracts a specific field from .data using a jq path.
# Example: json_field '.services.web.port'
json_field() {
  echo "$output" | jq -r ".data${1}"
}

#!/usr/bin/env bats
# Basic CLI tests — version, help, unknown commands.

load test_helper

@test "outport --version prints version string" {
  run "$OUTPORT" --version
  assert_success
  assert_output --partial "outport version"
}

@test "outport version prints version info" {
  run "$OUTPORT" version
  assert_success
  assert_output --partial "outport version"
}

@test "outport version --json returns envelope with version data" {
  run "$OUTPORT" version --json
  assert_success
  assert_json_ok
  local ver
  ver="$(json_field '.version')"
  [[ -n "$ver" ]]
  [[ "$ver" != "null" ]]
}

@test "outport with no args shows help" {
  run "$OUTPORT"
  assert_success
  assert_output --partial "Usage:"
  assert_output --partial "outport [command]"
}

@test "outport nonexistent shows error" {
  run "$OUTPORT" nonexistent
  assert_failure
  assert_output --partial "unknown command"
}

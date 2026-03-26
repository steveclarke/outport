#!/usr/bin/env bats
# Error handling tests — clear errors, JSON envelopes, validation.

load test_helper

@test "no outport.yml gives clear error" {
  # Remove the default outport.yml (setup doesn't create one)
  run "$OUTPORT" up
  assert_failure
  assert_output --partial "No outport.yml"
}

@test "--json errors return the error envelope format" {
  run "$OUTPORT" up --json
  assert_failure
  assert_json_error

  local error_msg
  error_msg="$(echo "$output" | jq -r '.error')"
  [[ -n "$error_msg" ]]
  [[ "$error_msg" != "null" ]]
  [[ "$error_msg" == *"outport.yml"* ]]
}

@test "invalid config gives a validation error" {
  create_config 'name: testapp
services:'
  run "$OUTPORT" up
  assert_failure
  assert_output --partial "No services defined"
}

@test "invalid config --json gives error envelope" {
  create_config 'name: testapp
services:'
  run "$OUTPORT" up --json
  assert_failure
  assert_json_error

  local error_msg
  error_msg="$(echo "$output" | jq -r '.error')"
  [[ "$error_msg" == *"No services defined"* ]]
}

@test "status --json on unregistered project gives error envelope" {
  # No config at all — loadProjectContext fails
  run "$OUTPORT" status --json
  assert_failure
  assert_json_error
}

@test "down on unregistered project gives error" {
  create_config 'name: testapp
services:
  web:
    env_var: PORT'
  run "$OUTPORT" down
  assert_failure
  assert_output --partial "not registered"
}

@test "down --json on unregistered project gives error envelope" {
  create_config 'name: testapp
services:
  web:
    env_var: PORT'
  run "$OUTPORT" down --json
  assert_failure
  assert_json_error

  local error_msg
  error_msg="$(echo "$output" | jq -r '.error')"
  [[ "$error_msg" == *"not registered"* ]]
}

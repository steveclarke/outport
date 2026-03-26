#!/usr/bin/env bats
# Tests for status and doctor commands.

load test_helper

BASIC_CONFIG='name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT'

@test "outport status --json after up shows services" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" up
  assert_success

  run "$OUTPORT" status --json
  assert_success
  assert_json_ok

  local project
  project="$(json_field '.project')"
  [[ "$project" == "testapp" ]]

  local web_port
  web_port="$(json_field '.services.web.port')"
  [[ "$web_port" -gt 0 ]]
}

@test "outport status with no project registered shows message" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" status
  assert_success
  assert_output --partial "No ports allocated"
}

@test "outport doctor --json returns envelope with results array" {
  run "$OUTPORT" doctor --json
  # doctor may exit non-zero if checks fail, that's OK
  assert_json_ok

  # Verify results array exists
  local count
  count="$(echo "$output" | jq '.data.results | length')"
  [[ "$count" -gt 0 ]]

  # Verify each result has expected fields
  local first_status first_name
  first_status="$(echo "$output" | jq -r '.data.results[0].status')"
  first_name="$(echo "$output" | jq -r '.data.results[0].name')"
  [[ -n "$first_status" ]]
  [[ "$first_status" != "null" ]]
  [[ -n "$first_name" ]]
  [[ "$first_name" != "null" ]]
}

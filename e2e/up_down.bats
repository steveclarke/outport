#!/usr/bin/env bats
# Tests for the up/down workflow — the core of Outport.

load test_helper

BASIC_CONFIG='name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT'

@test "outport up allocates ports" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" up --json
  assert_success
  assert_json_ok

  local project instance
  project="$(json_field '.project')"
  instance="$(json_field '.instance')"
  [[ "$project" == "testapp" ]]
  [[ "$instance" == "main" ]]

  # Both services should have non-zero ports
  local web_port pg_port
  web_port="$(json_field '.services.web.port')"
  pg_port="$(json_field '.services.postgres.port')"
  [[ "$web_port" -gt 0 ]]
  [[ "$pg_port" -gt 0 ]]
  [[ "$web_port" -ne "$pg_port" ]]
}

@test "outport up --json returns envelope with services data" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" up --json
  assert_success
  assert_json_ok

  # Verify the envelope structure
  local summary
  summary="$(echo "$output" | jq -r '.summary')"
  [[ "$summary" == "2 services allocated" ]]

  # Verify service fields
  local env_var
  env_var="$(json_field '.services.web.env_var')"
  [[ "$env_var" == "PORT" ]]

  env_var="$(json_field '.services.postgres.env_var')"
  [[ "$env_var" == "DATABASE_PORT" ]]
}

@test "outport up is idempotent" {
  create_config "$BASIC_CONFIG"

  run "$OUTPORT" up --json
  assert_success
  local web1 pg1
  web1="$(json_field '.services.web.port')"
  pg1="$(json_field '.services.postgres.port')"

  run "$OUTPORT" up --json
  assert_success
  local web2 pg2
  web2="$(json_field '.services.web.port')"
  pg2="$(json_field '.services.postgres.port')"

  [[ "$web1" == "$web2" ]]
  [[ "$pg1" == "$pg2" ]]
}

@test "outport up writes .env file with fenced block" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" up
  assert_success

  [[ -f .env ]]
  grep -q "# --- begin outport.dev ---" .env
  grep -q "PORT=" .env
  grep -q "DATABASE_PORT=" .env
  grep -q "# --- end outport.dev ---" .env
}

@test "outport down removes the registration" {
  create_config "$BASIC_CONFIG"
  run "$OUTPORT" up
  assert_success

  run "$OUTPORT" down --json
  assert_success
  assert_json_ok

  local status_val
  status_val="$(json_field '.status')"
  [[ "$status_val" == "removed" ]]
}

@test "outport down removes fenced block from .env" {
  create_config "$BASIC_CONFIG"

  # Add user content before running up
  echo "MY_VAR=hello" > .env

  run "$OUTPORT" up
  assert_success
  # Verify fenced block exists
  grep -q "# --- begin outport.dev ---" .env

  run "$OUTPORT" down
  assert_success
  # Fenced block should be gone
  ! grep -q "# --- begin outport.dev ---" .env
  # User content should remain
  grep -q "MY_VAR=hello" .env
}

@test "outport up then down is a clean round-trip" {
  create_config "$BASIC_CONFIG"

  run "$OUTPORT" up --json
  assert_success
  assert_json_ok

  run "$OUTPORT" down --json
  assert_success
  assert_json_ok

  local project
  project="$(json_field '.project')"
  [[ "$project" == "testapp" ]]

  # .env file should be empty or not exist
  if [[ -f .env ]]; then
    ! grep -q "PORT=" .env
    ! grep -q "DATABASE_PORT=" .env
  fi
}

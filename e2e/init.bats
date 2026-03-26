#!/usr/bin/env bats
# Tests for the init command.

load test_helper

@test "outport init creates outport.yml" {
  run "$OUTPORT" init
  assert_success
  assert_output --partial "Created outport.yml"
  [[ -f outport.yml ]]
}

@test "outport init fails if outport.yml already exists" {
  echo "name: existing" > outport.yml
  run "$OUTPORT" init
  assert_failure
  assert_output --partial "already exists"
}

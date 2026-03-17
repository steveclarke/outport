package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestAllCommandsHaveArgsValidation ensures every user-facing command
// has an Args validator set. Without this, commands silently ignore
// unexpected arguments, which is a poor user experience.
func TestAllCommandsHaveArgsValidation(t *testing.T) {
	// Skip hidden commands, Cobra's auto-generated help command, and parent commands
	skip := map[string]bool{
		"daemon":     true,
		"help":       true,
		"completion": true,
		"system":     true,
	}

	var allCmds []*cobra.Command
	for _, cmd := range rootCmd.Commands() {
		allCmds = append(allCmds, cmd)
		for _, sub := range cmd.Commands() {
			allCmds = append(allCmds, sub)
		}
	}
	for _, cmd := range allCmds {
		if skip[cmd.Name()] {
			continue
		}
		if cmd.Args == nil {
			t.Errorf("command %q has no Args validator — add NoArgs, ExactArgs, or MaximumArgs from cmdutil.go", cmd.Name())
		}
	}
}

// TestNoArgsCommandsRejectArguments verifies that commands which don't
// accept arguments return a FlagError when given unexpected args.
func TestNoArgsCommandsRejectArguments(t *testing.T) {
	noArgsCmds := []string{
		"apply", "gc", "init", "ports", "promote",
		"setup", "teardown", "status", "unapply", "up", "down",
	}

	for _, name := range noArgsCmds {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("command %q not found: %v", name, err)
		}

		validateErr := cmd.Args(cmd, []string{"unexpected-arg"})
		if validateErr == nil {
			t.Errorf("command %q accepted unexpected arguments", name)
			continue
		}
		if !IsFlagError(validateErr) {
			t.Errorf("command %q returned a plain error instead of FlagError: %v", name, validateErr)
		}
	}
}

// TestExactArgsCommands verifies commands that require specific arg counts.
func TestExactArgsCommands(t *testing.T) {
	tests := []struct {
		name     string
		tooFew   []string
		tooMany  []string
		justRight []string
	}{
		{
			name:      "rename",
			tooFew:    []string{"one"},
			tooMany:   []string{"a", "b", "c"},
			justRight: []string{"old", "new"},
		},
	}

	for _, tt := range tests {
		cmd, _, err := rootCmd.Find([]string{tt.name})
		if err != nil {
			t.Fatalf("command %q not found: %v", tt.name, err)
		}

		if err := cmd.Args(cmd, tt.tooFew); err == nil {
			t.Errorf("%s: accepted too few args %v", tt.name, tt.tooFew)
		} else if !IsFlagError(err) {
			t.Errorf("%s: too-few error is not FlagError: %v", tt.name, err)
		}

		if err := cmd.Args(cmd, tt.tooMany); err == nil {
			t.Errorf("%s: accepted too many args %v", tt.name, tt.tooMany)
		} else if !IsFlagError(err) {
			t.Errorf("%s: too-many error is not FlagError: %v", tt.name, err)
		}

		if err := cmd.Args(cmd, tt.justRight); err != nil {
			t.Errorf("%s: rejected correct args %v: %v", tt.name, tt.justRight, err)
		}
	}
}

// TestMaximumArgsCommands verifies commands that accept optional args.
func TestMaximumArgsCommands(t *testing.T) {
	tests := []struct {
		name    string
		valid   [][]string
		invalid []string
	}{
		{
			name:    "open",
			valid:   [][]string{{}, {"rails"}},
			invalid: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		cmd, _, err := rootCmd.Find([]string{tt.name})
		if err != nil {
			t.Fatalf("command %q not found: %v", tt.name, err)
		}

		for _, args := range tt.valid {
			if err := cmd.Args(cmd, args); err != nil {
				t.Errorf("%s: rejected valid args %v: %v", tt.name, args, err)
			}
		}

		if err := cmd.Args(cmd, tt.invalid); err == nil {
			t.Errorf("%s: accepted too many args %v", tt.name, tt.invalid)
		} else if !IsFlagError(err) {
			t.Errorf("%s: too-many error is not FlagError: %v", tt.name, err)
		}
	}
}

// testArgsValidator is a helper for testing cobra.PositionalArgs functions directly.
func testArgsValidator(t *testing.T, validator cobra.PositionalArgs, args []string, expectErr bool, expectFlagErr bool) {
	t.Helper()
	err := validator(&cobra.Command{}, args)
	if expectErr && err == nil {
		t.Errorf("expected error for args %v, got nil", args)
	}
	if !expectErr && err != nil {
		t.Errorf("unexpected error for args %v: %v", args, err)
	}
	if expectFlagErr && err != nil && !IsFlagError(err) {
		t.Errorf("expected FlagError for args %v, got plain error: %v", args, err)
	}
}

func TestExactArgsHelper(t *testing.T) {
	v := ExactArgs(2, "need two args")
	testArgsValidator(t, v, []string{"a", "b"}, false, false)
	testArgsValidator(t, v, []string{"a"}, true, true)
	testArgsValidator(t, v, []string{}, true, true)
	testArgsValidator(t, v, []string{"a", "b", "c"}, true, true)
}

func TestNoArgsHelper(t *testing.T) {
	testArgsValidator(t, NoArgs, []string{}, false, false)
	testArgsValidator(t, NoArgs, []string{"foo"}, true, true)
}

func TestMaximumArgsHelper(t *testing.T) {
	v := MaximumArgs(1, "too many")
	testArgsValidator(t, v, []string{}, false, false)
	testArgsValidator(t, v, []string{"a"}, false, false)
	testArgsValidator(t, v, []string{"a", "b"}, true, true)
}

func TestMinimumArgsHelper(t *testing.T) {
	v := MinimumArgs(1, "need at least one")
	testArgsValidator(t, v, []string{"a"}, false, false)
	testArgsValidator(t, v, []string{"a", "b"}, false, false)
	testArgsValidator(t, v, []string{}, true, true)
}

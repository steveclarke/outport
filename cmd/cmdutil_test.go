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
		allCmds = append(allCmds, cmd.Commands()...)
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
		"up", "down", "init", "ports", "promote",
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

// TestRangeArgsCommands verifies commands that accept a range of args.
func TestRangeArgsCommands(t *testing.T) {
	tests := []struct {
		name    string
		valid   [][]string
		invalid [][]string
	}{
		{
			name:    "rename",
			valid:   [][]string{{"new"}, {"old", "new"}},
			invalid: [][]string{{}, {"a", "b", "c"}},
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

		for _, args := range tt.invalid {
			if err := cmd.Args(cmd, args); err == nil {
				t.Errorf("%s: accepted invalid args %v", tt.name, args)
			} else if !IsFlagError(err) {
				t.Errorf("%s: error for args %v is not FlagError: %v", tt.name, args, err)
			}
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

func TestRangeArgsHelper(t *testing.T) {
	v := RangeArgs(1, 2, "requires 1 or 2 args")
	testArgsValidator(t, v, []string{"a"}, false, false)
	testArgsValidator(t, v, []string{"a", "b"}, false, false)
	testArgsValidator(t, v, []string{}, true, true)
	testArgsValidator(t, v, []string{"a", "b", "c"}, true, true)
}

func TestSystemCommandHasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"system"})
	if err != nil {
		t.Fatalf("system command not found: %v", err)
	}
	if !cmd.HasSubCommands() {
		t.Error("system command should have subcommands")
	}
}

func TestSystemSubcommandsRejectArguments(t *testing.T) {
	subCmds := []string{"start", "stop", "restart", "status", "gc", "uninstall"}

	for _, name := range subCmds {
		cmd, _, err := rootCmd.Find([]string{"system", name})
		if err != nil {
			t.Errorf("command system %q not found: %v", name, err)
			continue
		}

		validateErr := cmd.Args(cmd, []string{"unexpected-arg"})
		if validateErr == nil {
			t.Errorf("command system %q accepted unexpected arguments", name)
			continue
		}
		if !IsFlagError(validateErr) {
			t.Errorf("command system %q returned a plain error instead of FlagError: %v", name, validateErr)
		}
	}
}

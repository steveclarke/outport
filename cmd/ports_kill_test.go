package cmd

import (
	"strings"
	"testing"
)

func TestPortsKillCommand_Registered(t *testing.T) {
	found := false
	for _, cmd := range portsCmd.Commands() {
		if cmd.Name() == "kill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("kill subcommand not registered on portsCmd")
	}
}

func TestPortsKillCommand_HasOrphansFlag(t *testing.T) {
	flag := portsKillCmd.Flags().Lookup("orphans")
	if flag == nil {
		t.Fatal("kill command missing --orphans flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--orphans default = %q, want %q", flag.DefValue, "false")
	}
}

func TestPortsKillCommand_HasForceFlag(t *testing.T) {
	flag := portsKillCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("kill command missing --force flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--force default = %q, want %q", flag.DefValue, "false")
	}
}

func TestPortsKillCommand_RequiresTarget(t *testing.T) {
	// With no --orphans and no args, the arg validator should return an error
	err := portsKillCmd.Args(portsKillCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no target and no --orphans")
	}
	if !IsFlagError(err) {
		t.Errorf("expected FlagError, got %T: %v", err, err)
	}
}

func TestPortsKillCommand_RejectsTooManyArgs(t *testing.T) {
	err := portsKillCmd.Args(portsKillCmd, []string{"3000", "4000"})
	if err == nil {
		t.Fatal("expected error with too many args")
	}
	if !IsFlagError(err) {
		t.Errorf("expected FlagError, got %T: %v", err, err)
	}
}

func TestResolveKillTarget_PortNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"valid port", "3000", 3000, false},
		{"min port", "1", 1, false},
		{"max port", "65535", 65535, false},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"too high", "99999", 0, true},
		{"not a number", "abc", 0, true}, // will fail without project context
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveKillTarget(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveKillTarget(%q) = %d, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveKillTarget(%q) error = %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("resolveKillTarget(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfirmYes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"lowercase y", "y\n", true},
		{"uppercase Y", "Y\n", true},
		{"yes", "yes\n", true},
		{"YES", "YES\n", true},
		{"Yes", "Yes\n", true},
		{"no", "n\n", false},
		{"No", "No\n", false},
		{"empty", "\n", false},
		{"other", "maybe\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			got := confirmYes(r)
			if got != tt.want {
				t.Errorf("confirmYes(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatKillCandidate(t *testing.T) {
	// Just verify it doesn't panic and produces non-empty output
	var buf strings.Builder
	printKillCandidate(&buf, killCandidate{
		port: 3000,
		pid:  1234,
		name: "node",
		cmd:  "node server.js",
	})
	output := buf.String()
	if output == "" {
		t.Fatal("printKillCandidate produced empty output")
	}
	if !strings.Contains(output, "3000") {
		t.Error("output should contain the port number")
	}
	if !strings.Contains(output, "1234") {
		t.Error("output should contain the PID")
	}
}

//go:build darwin

package platform

import (
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	binary := "/usr/local/bin/outport"
	plist := GeneratePlist(binary)

	checks := []struct {
		name     string
		contains string
	}{
		{"label", "<string>dev.outport.daemon</string>"},
		{"binary path", "<string>/usr/local/bin/outport</string>"},
		{"daemon subcommand", "<string>daemon</string>"},
		{"RunAtLoad", "<key>RunAtLoad</key>"},
		{"KeepAlive", "<key>KeepAlive</key>"},
		{"socket node", "<string>127.0.0.1</string>"},
		{"socket port", "<string>80</string>"},
		{"SockNodeName key", "<key>SockNodeName</key>"},
		{"SockServiceName key", "<key>SockServiceName</key>"},
		{"log path", "<string>/tmp/outport-daemon.log</string>"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(plist, tc.contains) {
				t.Errorf("plist missing %q:\n%s", tc.contains, plist)
			}
		})
	}
}

func TestGeneratePlistDifferentBinary(t *testing.T) {
	binary := "/opt/homebrew/bin/outport"
	plist := GeneratePlist(binary)

	if !strings.Contains(plist, "<string>/opt/homebrew/bin/outport</string>") {
		t.Error("plist does not contain the specified binary path")
	}
}

func TestIsSetup(t *testing.T) {
	// IsSetup should return false in a test environment where neither
	// the resolver file nor the plist are installed.
	// We can't guarantee the state of the machine, but we can verify
	// the function doesn't panic and returns a bool.
	_ = IsSetup()
}

func TestPlistPath(t *testing.T) {
	path := plistPath()
	if path == "" {
		t.Fatal("plistPath() returned empty string")
	}
	if !strings.Contains(path, "Library/LaunchAgents") {
		t.Errorf("plistPath() = %q, want it to contain Library/LaunchAgents", path)
	}
	if !strings.HasSuffix(path, plistName) {
		t.Errorf("plistPath() = %q, want it to end with %s", path, plistName)
	}
}

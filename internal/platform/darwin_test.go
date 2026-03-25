//go:build darwin

package platform

import (
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	binary := "/usr/local/bin/outport"
	plist := GeneratePlist(binary, 80, 443)

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
	plist := GeneratePlist(binary, 80, 443)

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

func TestGeneratePlistDualSockets(t *testing.T) {
	plist := GeneratePlist("/usr/local/bin/outport", 80, 443)
	if !strings.Contains(plist, "<key>HTTPSocket</key>") {
		t.Error("plist missing HTTPSocket key")
	}
	if !strings.Contains(plist, "<key>HTTPSSocket</key>") {
		t.Error("plist missing HTTPSSocket key")
	}
	if !strings.Contains(plist, "<string>443</string>") {
		t.Error("plist missing port 443")
	}
	if strings.Contains(plist, `<key>Socket</key>`) {
		t.Error("plist still has old Socket key")
	}
}

func TestGeneratePlistCustomPorts(t *testing.T) {
	plist := GeneratePlist("/usr/local/bin/outport", 8080, 8443)

	if !strings.Contains(plist, "<string>8080</string>") {
		t.Error("plist missing custom HTTP port 8080")
	}
	if !strings.Contains(plist, "<string>8443</string>") {
		t.Error("plist missing custom HTTPS port 8443")
	}
	if strings.Contains(plist, "<string>80</string>") {
		t.Error("plist still has default port 80")
	}
}

func TestPlistPath(t *testing.T) {
	path := PlistPath()
	if path == "" {
		t.Fatal("PlistPath() returned empty string")
	}
	if !strings.Contains(path, "Library/LaunchAgents") {
		t.Errorf("PlistPath() = %q, want it to contain Library/LaunchAgents", path)
	}
	if !strings.HasSuffix(path, plistName) {
		t.Errorf("PlistPath() = %q, want it to end with %s", path, plistName)
	}
}

//go:build linux

package platform

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratePlist(t *testing.T) {
	binary := "/usr/local/bin/outport"
	unit := GeneratePlist(binary)

	checks := []struct {
		name     string
		contains string
	}{
		{"description", "Description=Outport development proxy daemon"},
		{"binary path", "ExecStart=/usr/local/bin/outport daemon"},
		{"restart policy", "Restart=on-failure"},
		{"install target", "WantedBy=default.target"},
		{"after network", "After=network.target"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(unit, tc.contains) {
				t.Errorf("unit missing %q:\n%s", tc.contains, unit)
			}
		})
	}
}

func TestGeneratePlistDifferentBinary(t *testing.T) {
	binary := "/opt/outport/bin/outport"
	unit := GeneratePlist(binary)

	if !strings.Contains(unit, "ExecStart=/opt/outport/bin/outport daemon") {
		t.Error("unit does not contain the specified binary path")
	}
}

func TestPlistPath(t *testing.T) {
	path := PlistPath()
	if path == "" {
		t.Fatal("PlistPath() returned empty string")
	}
	if !strings.Contains(path, ".config/systemd/user") {
		t.Errorf("PlistPath() = %q, want it to contain .config/systemd/user", path)
	}
	if !strings.HasSuffix(path, serviceFile) {
		t.Errorf("PlistPath() = %q, want it to end with %s", path, serviceFile)
	}
}

func TestResolverConstants(t *testing.T) {
	if !strings.Contains(ResolverPath, "resolved.conf.d") {
		t.Errorf("ResolverPath = %q, want it to contain resolved.conf.d", ResolverPath)
	}
	if !strings.Contains(ResolverContent, "DNS=127.0.0.1:15353") {
		t.Errorf("ResolverContent missing DNS server config")
	}
	if !strings.Contains(ResolverContent, "Domains=~test") {
		t.Errorf("ResolverContent missing domain routing")
	}
}

func TestWritePlistCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path := PlistPath()
	if path == "" {
		t.Fatal("PlistPath() returned empty after setting HOME")
	}

	err := WritePlist("/usr/local/bin/outport")
	if err != nil {
		t.Fatalf("WritePlist() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading service file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ExecStart=/usr/local/bin/outport daemon") {
		t.Error("service file missing ExecStart line")
	}
}

func TestRemovePlist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_ = WritePlist("/usr/local/bin/outport")

	path := PlistPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("service file should exist after WritePlist")
	}

	if err := RemovePlist(); err != nil {
		t.Fatalf("RemovePlist() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("service file should not exist after RemovePlist")
	}
}

func TestRemovePlistNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if err := RemovePlist(); err != nil {
		t.Fatalf("RemovePlist() should not error for missing file: %v", err)
	}
}

func TestDetectCATrust(t *testing.T) {
	cfg, err := detectCATrust()
	if err != nil {
		t.Skipf("CA trust detection failed (expected in some environments): %v", err)
	}
	if cfg.certDir == "" {
		t.Error("detected CA trust config has empty certDir")
	}
	if cfg.updateCmd == "" {
		t.Error("detected CA trust config has empty updateCmd")
	}
}

func TestIsPlistInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if isPlistInstalled() {
		t.Error("isPlistInstalled() should be false before WritePlist")
	}

	_ = WritePlist("/usr/local/bin/outport")
	if !isPlistInstalled() {
		t.Error("isPlistInstalled() should be true after WritePlist")
	}
}

func TestServiceDescription(t *testing.T) {
	desc := ServiceDescription()
	if desc != "systemd service" {
		t.Errorf("ServiceDescription() = %q, want %q", desc, "systemd service")
	}
}

func TestResolverDescription(t *testing.T) {
	desc := ResolverDescription()
	if desc != "systemd-resolved config" {
		t.Errorf("ResolverDescription() = %q, want %q", desc, "systemd-resolved config")
	}
}

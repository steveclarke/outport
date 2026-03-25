package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := Defaults()
	if s.Dashboard.HealthInterval != d.Dashboard.HealthInterval {
		t.Errorf("HealthInterval = %v, want %v", s.Dashboard.HealthInterval, d.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != d.DNS.TTL {
		t.Errorf("TTL = %d, want %d", s.DNS.TTL, d.DNS.TTL)
	}
}

func TestLoadEmptyFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := Defaults()
	if s.Dashboard.HealthInterval != d.Dashboard.HealthInterval {
		t.Errorf("HealthInterval = %v, want %v", s.Dashboard.HealthInterval, d.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != d.DNS.TTL {
		t.Errorf("TTL = %d, want %d", s.DNS.TTL, d.DNS.TTL)
	}
}

func TestLoadFullFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dashboard]
health_interval = 5s

[dns]
ttl = 30
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Dashboard.HealthInterval != 5*time.Second {
		t.Errorf("HealthInterval = %v, want 5s", s.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != 30 {
		t.Errorf("TTL = %d, want 30", s.DNS.TTL)
	}
}

func TestLoadPartialFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dashboard]
health_interval = 10s
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := Defaults()
	if s.Dashboard.HealthInterval != 10*time.Second {
		t.Errorf("HealthInterval = %v, want 10s", s.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != d.DNS.TTL {
		t.Errorf("TTL = %d, want %d (default)", s.DNS.TTL, d.DNS.TTL)
	}
}

func TestLoadPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `# Outport global settings
# This is a comment

[dashboard]
# health_interval = 3s
health_interval = 10s

[dns]
ttl = 45
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Dashboard.HealthInterval != 10*time.Second {
		t.Errorf("HealthInterval = %v, want 10s", s.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != 45 {
		t.Errorf("TTL = %d, want 45", s.DNS.TTL)
	}
}

func TestLoadInvalidHealthIntervalTooShort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dashboard]
health_interval = 500ms
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for health_interval 500ms, got nil")
	}
}

func TestLoadInvalidHealthIntervalBadFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dashboard]
health_interval = banana
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for health_interval 'banana', got nil")
	}
}

func TestLoadInvalidTTLNegative(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dns]
ttl = -1
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for ttl -1, got nil")
	}
}

func TestLoadInvalidTTLZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[dns]
ttl = 0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for ttl 0, got nil")
	}
}

func TestDefaultsReturnsExpectedValues(t *testing.T) {
	d := Defaults()

	if d.Dashboard.HealthInterval != 3*time.Second {
		t.Errorf("HealthInterval = %v, want 3s", d.Dashboard.HealthInterval)
	}
	if d.DNS.TTL != 60 {
		t.Errorf("TTL = %d, want 60", d.DNS.TTL)
	}
}

func TestDefaultConfigContentRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(DefaultConfigContent()), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("DefaultConfigContent produced invalid config: %v", err)
	}
	d := Defaults()
	if s.Dashboard.HealthInterval != d.Dashboard.HealthInterval {
		t.Errorf("HealthInterval = %v, want %v", s.Dashboard.HealthInterval, d.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != d.DNS.TTL {
		t.Errorf("TTL = %d, want %d", s.DNS.TTL, d.DNS.TTL)
	}
}

func TestPathReturnsConfigOutportConfig(t *testing.T) {
	p, err := Path()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := filepath.Base(p)
	if base != "config" {
		t.Errorf("base = %q, want %q", base, "config")
	}

	parent := filepath.Base(filepath.Dir(p))
	if parent != "outport" {
		t.Errorf("parent dir = %q, want %q", parent, "outport")
	}
}

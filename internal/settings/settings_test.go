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
	if s.Proxy.HTTPPort != d.Proxy.HTTPPort {
		t.Errorf("HTTPPort = %d, want %d", s.Proxy.HTTPPort, d.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != d.Proxy.HTTPSPort {
		t.Errorf("HTTPSPort = %d, want %d", s.Proxy.HTTPSPort, d.Proxy.HTTPSPort)
	}
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
	if s.Proxy.HTTPPort != d.Proxy.HTTPPort {
		t.Errorf("HTTPPort = %d, want %d", s.Proxy.HTTPPort, d.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != d.Proxy.HTTPSPort {
		t.Errorf("HTTPSPort = %d, want %d", s.Proxy.HTTPSPort, d.Proxy.HTTPSPort)
	}
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
	content := `[proxy]
http_port = 8080
https_port = 8443

[dashboard]
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

	if s.Proxy.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", s.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != 8443 {
		t.Errorf("HTTPSPort = %d, want 8443", s.Proxy.HTTPSPort)
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
	content := `[proxy]
http_port = 9090
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := Defaults()
	if s.Proxy.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d, want 9090", s.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != d.Proxy.HTTPSPort {
		t.Errorf("HTTPSPort = %d, want %d (default)", s.Proxy.HTTPSPort, d.Proxy.HTTPSPort)
	}
	if s.Dashboard.HealthInterval != d.Dashboard.HealthInterval {
		t.Errorf("HealthInterval = %v, want %v (default)", s.Dashboard.HealthInterval, d.Dashboard.HealthInterval)
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

[proxy]
# http_port = 80
http_port = 7070
https_port = 7443

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

	if s.Proxy.HTTPPort != 7070 {
		t.Errorf("HTTPPort = %d, want 7070", s.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != 7443 {
		t.Errorf("HTTPSPort = %d, want 7443", s.Proxy.HTTPSPort)
	}
	if s.Dashboard.HealthInterval != 10*time.Second {
		t.Errorf("HealthInterval = %v, want 10s", s.Dashboard.HealthInterval)
	}
	if s.DNS.TTL != 45 {
		t.Errorf("TTL = %d, want 45", s.DNS.TTL)
	}
}

func TestLoadInvalidPortTooHigh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[proxy]
http_port = 70000
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for port 70000, got nil")
	}
}

func TestLoadInvalidPortZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := `[proxy]
http_port = 0
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for port 0, got nil")
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

	if d.Proxy.HTTPPort != 80 {
		t.Errorf("HTTPPort = %d, want 80", d.Proxy.HTTPPort)
	}
	if d.Proxy.HTTPSPort != 443 {
		t.Errorf("HTTPSPort = %d, want 443", d.Proxy.HTTPSPort)
	}
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
	if s.Proxy.HTTPPort != d.Proxy.HTTPPort {
		t.Errorf("HTTPPort = %d, want %d", s.Proxy.HTTPPort, d.Proxy.HTTPPort)
	}
	if s.Proxy.HTTPSPort != d.Proxy.HTTPSPort {
		t.Errorf("HTTPSPort = %d, want %d", s.Proxy.HTTPSPort, d.Proxy.HTTPSPort)
	}
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

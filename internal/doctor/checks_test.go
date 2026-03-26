package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveclarke/outport/internal/platform"
)

func TestCheckResolverContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test")

	// Missing file
	res := checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Fail {
		t.Errorf("expected Fail for missing file, got %v", res.Status)
	}

	// Wrong content
	_ = os.WriteFile(path, []byte("wrong"), 0644)
	res = checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Fail {
		t.Errorf("expected Fail for wrong content, got %v", res.Status)
	}

	// Correct content
	_ = os.WriteFile(path, []byte("nameserver 127.0.0.1\nport 15353\n"), 0644)
	res = checkResolverContent(path, "nameserver 127.0.0.1\nport 15353\n")
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}
}

func TestCheckFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists")

	res := checkFileExists(path, "test file", "fix it")
	if res.Status != Fail {
		t.Errorf("expected Fail, got %v", res.Status)
	}

	_ = os.WriteFile(path, []byte("x"), 0644)
	res = checkFileExists(path, "test file", "fix it")
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}
}

func TestCheckPlistBinary(t *testing.T) {
	dir := t.TempDir()
	servicePath := filepath.Join(dir, "test.service")
	binaryPath := filepath.Join(dir, "outport")

	// Use platform.GeneratePlist so the fixture matches the current OS format
	// (plist XML on macOS, systemd unit on Linux)
	serviceContent := platform.GeneratePlist(binaryPath)
	_ = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	_ = os.WriteFile(binaryPath, []byte("binary"), 0755)

	res := checkPlistBinary(servicePath)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}

	// Binary doesn't exist
	os.Remove(binaryPath)
	res = checkPlistBinary(servicePath)
	if res.Status != Fail {
		t.Errorf("expected Fail for missing binary, got %v", res.Status)
	}

	// Malformed content
	_ = os.WriteFile(servicePath, []byte("garbage content"), 0644)
	res = checkPlistBinary(servicePath)
	if res.Status != Fail {
		t.Errorf("expected Fail for malformed service file, got %v", res.Status)
	}
}

func TestParsePlistBinaryPath(t *testing.T) {
	minimal := []byte(`<?xml version="1.0"?><plist><dict><key>ProgramArguments</key><array><string>/usr/local/bin/outport</string><string>daemon</string></array></dict></plist>`)
	if got := parsePlistBinaryPath(minimal); got != "/usr/local/bin/outport" {
		t.Errorf("expected /usr/local/bin/outport, got %q", got)
	}

	if got := parsePlistBinaryPath([]byte("")); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}

	noProg := []byte(`<?xml version="1.0"?><plist><dict><key>Label</key><string>test</string></dict></plist>`)
	if got := parsePlistBinaryPath(noProg); got != "" {
		t.Errorf("expected empty string for missing ProgramArguments, got %q", got)
	}
}

func TestCheckCertExpiry(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")

	res := checkCertExpiry(certPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for missing cert, got %v", res.Status)
	}

	_ = os.WriteFile(certPath, []byte("not a cert"), 0644)
	res = checkCertExpiry(certPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for invalid cert, got %v", res.Status)
	}
}

func TestCheckRegistryValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	res := checkRegistryValid(path)
	if res.Status != Warn {
		t.Errorf("expected Warn for missing registry, got %v", res.Status)
	}

	_ = os.WriteFile(path, []byte(`{"projects":{}}`), 0644)
	res = checkRegistryValid(path)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v", res.Status)
	}

	_ = os.WriteFile(path, []byte(`{broken`), 0644)
	res = checkRegistryValid(path)
	if res.Status != Fail {
		t.Errorf("expected Fail for corrupt registry, got %v", res.Status)
	}
}

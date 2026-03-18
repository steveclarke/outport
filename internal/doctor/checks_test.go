package doctor

import (
	"os"
	"path/filepath"
	"testing"
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
	plistPath := filepath.Join(dir, "test.plist")
	binaryPath := filepath.Join(dir, "outport")

	// Realistic plist matching platform.GeneratePlist() structure
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.outport.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + binaryPath + `</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>Sockets</key>
    <dict>
        <key>HTTPSocket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>80</string>
        </dict>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/outport-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/outport-daemon.log</string>
</dict>
</plist>`
	_ = os.WriteFile(plistPath, []byte(plist), 0644)
	_ = os.WriteFile(binaryPath, []byte("binary"), 0755)

	res := checkPlistBinary(plistPath)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}

	// Binary doesn't exist
	os.Remove(binaryPath)
	res = checkPlistBinary(plistPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for missing binary, got %v", res.Status)
	}

	// Malformed plist
	_ = os.WriteFile(plistPath, []byte("not xml"), 0644)
	res = checkPlistBinary(plistPath)
	if res.Status != Fail {
		t.Errorf("expected Fail for malformed plist, got %v", res.Status)
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

// internal/certmanager/ca_test.go
package certmanager

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")
	keyPath := filepath.Join(dir, "ca-key.pem")

	if err := GenerateCA(certPath, keyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	// Cert file exists and is valid
	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(certData)
	if block == nil {
		t.Fatal("no PEM block in cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	if !cert.IsCA {
		t.Error("cert is not a CA")
	}
	if cert.Subject.Organization[0] != "Outport Dev CA" {
		t.Errorf("org = %q, want %q", cert.Subject.Organization[0], "Outport Dev CA")
	}
	if cert.Subject.CommonName != "Outport Dev CA" {
		t.Errorf("CN = %q, want %q", cert.Subject.CommonName, "Outport Dev CA")
	}
	if !cert.MaxPathLenZero {
		t.Error("MaxPathLenZero should be true")
	}

	// Key file has 0600 permissions
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if keyInfo.Mode().Perm() != 0600 {
		t.Errorf("key permissions = %o, want 0600", keyInfo.Mode().Perm())
	}
}

func TestGenerateCAIdempotent(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")
	keyPath := filepath.Join(dir, "ca-key.pem")

	if err := GenerateCA(certPath, keyPath); err != nil {
		t.Fatalf("first GenerateCA: %v", err)
	}
	certData1, _ := os.ReadFile(certPath)

	if err := GenerateCA(certPath, keyPath); err != nil {
		t.Fatalf("second GenerateCA: %v", err)
	}
	certData2, _ := os.ReadFile(certPath)

	if string(certData1) != string(certData2) {
		t.Error("second GenerateCA overwrote existing cert")
	}
}

func TestLoadCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")
	keyPath := filepath.Join(dir, "ca-key.pem")

	GenerateCA(certPath, keyPath)

	cert, key, err := LoadCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}
	if cert == nil {
		t.Error("cert is nil")
	}
	if key == nil {
		t.Error("key is nil")
	}
	if !cert.IsCA {
		t.Error("loaded cert is not a CA")
	}
}

func TestLoadCAMissingFile(t *testing.T) {
	_, _, err := LoadCA("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error for missing files")
	}
}

func TestIsCAInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if IsCAInstalled() {
		t.Error("expected false when CA does not exist")
	}

	dataDir := filepath.Join(home, ".local", "share", "outport")
	os.MkdirAll(dataDir, 0755)
	GenerateCA(filepath.Join(dataDir, "ca-cert.pem"), filepath.Join(dataDir, "ca-key.pem"))

	if !IsCAInstalled() {
		t.Error("expected true when CA exists")
	}
}

func TestDeleteCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca-cert.pem")
	keyPath := filepath.Join(dir, "ca-key.pem")

	GenerateCA(certPath, keyPath)
	DeleteCA(certPath, keyPath)

	if _, err := os.Stat(certPath); !os.IsNotExist(err) {
		t.Error("cert file still exists after DeleteCA")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Error("key file still exists after DeleteCA")
	}
}

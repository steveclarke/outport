// internal/certmanager/store_test.go
package certmanager

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestCertStoreGetCertificate(t *testing.T) {
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	_ = GenerateCA(caCertPath, caKeyPath)
	cacheDir := filepath.Join(dir, "certs")

	store, err := NewCertStore(caCertPath, caKeyPath, cacheDir)
	if err != nil {
		t.Fatalf("NewCertStore: %v", err)
	}

	hello := &tls.ClientHelloInfo{ServerName: "myapp.test"}
	cert, err := store.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert == nil {
		t.Fatal("cert is nil")
	}

	// Second call should return from memory cache (same pointer)
	cert2, err := store.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate (cached): %v", err)
	}
	if cert != cert2 {
		t.Error("expected same pointer from memory cache")
	}
}

func TestCertStoreLoadFromDiskCache(t *testing.T) {
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	_ = GenerateCA(caCertPath, caKeyPath)
	cacheDir := filepath.Join(dir, "certs")

	// First store generates and caches to disk
	store1, _ := NewCertStore(caCertPath, caKeyPath, cacheDir)
	hello := &tls.ClientHelloInfo{ServerName: "app.test"}
	if _, err := store1.GetCertificate(hello); err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	// Second store (fresh memory, same disk cache) should load from disk
	store2, _ := NewCertStore(caCertPath, caKeyPath, cacheDir)
	cert, err := store2.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate from disk: %v", err)
	}
	if cert == nil {
		t.Fatal("cert is nil from disk")
	}

	if _, err := os.Stat(filepath.Join(cacheDir, "app.test.pem")); err != nil {
		t.Errorf("cert not on disk: %v", err)
	}
}

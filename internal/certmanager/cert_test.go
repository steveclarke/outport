// internal/certmanager/cert_test.go
package certmanager

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetOrCreateCert(t *testing.T) {
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")

	if err := GenerateCA(caCertPath, caKeyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	caCert, caKey, err := LoadCA(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}

	cacheDir := filepath.Join(dir, "certs")

	cert, err := GetOrCreateCert("myapp.test", caCert, caKey, cacheDir)
	if err != nil {
		t.Fatalf("GetOrCreateCert: %v", err)
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	// Check SAN
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "myapp.test" {
		t.Errorf("DNSNames = %v, want [myapp.test]", leaf.DNSNames)
	}

	// Check signed by our CA
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		t.Errorf("cert not signed by CA: %v", err)
	}

	// Check cached to disk
	certPath := filepath.Join(cacheDir, "myapp.test.pem")
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("cert not cached to disk: %v", err)
	}
	keyPath := filepath.Join(cacheDir, "myapp.test-key.pem")
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Errorf("key not cached to disk: %v", err)
	} else if keyInfo.Mode().Perm() != 0600 {
		t.Errorf("key permissions = %o, want 0600", keyInfo.Mode().Perm())
	}
}

func TestGetOrCreateCertCached(t *testing.T) {
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	if err := GenerateCA(caCertPath, caKeyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	caCert, caKey, _ := LoadCA(caCertPath, caKeyPath)
	cacheDir := filepath.Join(dir, "certs")

	cert1, _ := GetOrCreateCert("app.test", caCert, caKey, cacheDir)
	cert2, _ := GetOrCreateCert("app.test", caCert, caKey, cacheDir)

	if string(cert1.Certificate[0]) != string(cert2.Certificate[0]) {
		t.Error("second call should return cached cert")
	}
}

func TestCertExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca-cert.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	if err := GenerateCA(caCertPath, caKeyPath); err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	caCert, caKey, _ := LoadCA(caCertPath, caKeyPath)
	cacheDir := filepath.Join(dir, "certs")

	// Generate a cert that expires in 3 days (within 7-day renewal window)
	cert, err := generateServerCertWithExpiry("soon.test", caCert, caKey, 3*24*time.Hour)
	if err != nil {
		t.Fatalf("generateServerCertWithExpiry: %v", err)
	}
	if err := saveCertToDisk(cacheDir, "soon.test", cert); err != nil {
		t.Fatalf("saveCertToDisk: %v", err)
	}

	// GetOrCreateCert should regenerate it
	newCert, err := GetOrCreateCert("soon.test", caCert, caKey, cacheDir)
	if err != nil {
		t.Fatalf("GetOrCreateCert: %v", err)
	}
	newLeaf, _ := x509.ParseCertificate(newCert.Certificate[0])
	if time.Until(newLeaf.NotAfter) < 30*24*time.Hour {
		t.Error("cert was not regenerated despite expiring soon")
	}
}

func TestDeleteCertCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cacheDir := filepath.Join(home, ".cache", "outport", "certs")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "test.pem"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteCertCache(); err != nil {
		t.Fatalf("DeleteCertCache: %v", err)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("cache dir still exists after DeleteCertCache")
	}
}

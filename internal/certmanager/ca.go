// Package certmanager handles the local certificate authority (CA) and per-hostname
// TLS certificate lifecycle for Outport's HTTPS proxy. It generates a self-signed
// root CA that gets trusted in the macOS keychain (via the platform package), then
// issues short-lived server certificates on demand for each .test hostname. Certificates
// are cached to disk under ~/.cache/outport/certs/ so they survive daemon restarts,
// and are regenerated automatically when they approach expiration or when the CA changes.
package certmanager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// GenerateCA creates a new root Certificate Authority certificate and private key,
// writing them as PEM files to certPath and keyPath respectively. If both files
// already exist on disk, GenerateCA returns immediately without overwriting them,
// making it safe to call on every daemon startup (idempotent).
//
// The CA uses an ECDSA P-256 key and a 10-year validity period. It is configured
// as a root CA (self-signed, IsCA=true) with MaxPathLen=0, meaning it can sign
// server certificates but those certificates cannot themselves act as CAs.
//
// The generated CA is what gets added to the macOS login keychain via platform.TrustCA,
// allowing browsers and other TLS clients to accept the server certificates it signs.
func GenerateCA(certPath, keyPath string) error {
	if fileExists(certPath) && fileExists(keyPath) {
		return nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Outport Dev CA"},
			CommonName:   "Outport Dev CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("creating CA certificate: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("creating cert directory: %w", err)
	}

	if err := writeCertPEM(certPath, certDER); err != nil {
		return fmt.Errorf("encoding cert PEM: %w", err)
	}

	if err := writeKeyPEM(keyPath, key); err != nil {
		return fmt.Errorf("encoding key PEM: %w", err)
	}

	return nil
}

// LoadCA reads the CA certificate and private key from their PEM files on disk
// and returns the parsed objects. It is used by CertStore and GetOrCreateCert to
// obtain the CA credentials needed to sign new server certificates. Returns an
// error if either file is missing, malformed, or not valid PEM.
func LoadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA cert: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("no PEM block in CA cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA cert: %w", err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading CA key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("no PEM block in CA key")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CA key: %w", err)
	}

	return cert, key, nil
}

// DeleteCA removes the CA certificate and private key files from disk. This is
// used during teardown (e.g., "outport system teardown") to clean up the local CA.
// After calling DeleteCA, any cached server certificates signed by this CA will
// fail verification and be regenerated on next use. Errors are silently ignored
// because the files may not exist.
func DeleteCA(certPath, keyPath string) {
	os.Remove(certPath)
	os.Remove(keyPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

package certmanager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// renewalWindow is how close to expiration a cached certificate can be before
// it is considered stale and regenerated. A 7-day window ensures certificates
// are renewed well before they actually expire, avoiding TLS errors for users
// who may not restart the daemon frequently.
const renewalWindow = 7 * 24 * time.Hour

// GetOrCreateCert returns a TLS certificate for the given hostname, suitable for
// serving HTTPS traffic. It implements a two-tier caching strategy:
//
//  1. Disk cache: Looks for existing PEM files in cacheDir named "{hostname}.pem"
//     and "{hostname}-key.pem". If found and still valid (not expiring within the
//     renewal window, and signed by the current CA), the cached certificate is returned.
//  2. Generation: If no valid cached certificate exists, a new server certificate is
//     generated, signed by the provided CA, saved to cacheDir, and returned.
//
// This function is called by CertStore (which adds an in-memory cache layer on top)
// and can also be called directly when memory caching is not needed.
func GetOrCreateCert(hostname string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, cacheDir string) (*tls.Certificate, error) {
	certPath := filepath.Join(cacheDir, hostname+".pem")
	keyPath := filepath.Join(cacheDir, hostname+"-key.pem")

	if cert, err := loadCertFromDisk(certPath, keyPath, caCert); err == nil {
		return cert, nil
	}

	cert, err := generateServerCert(hostname, caCert, caKey)
	if err != nil {
		return nil, err
	}

	if err := saveCertToDisk(cacheDir, hostname, cert); err != nil {
		return nil, err
	}

	return cert, nil
}

func loadCertFromDisk(certPath, keyPath string, caCert *x509.Certificate) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	if time.Until(leaf.NotAfter) < renewalWindow {
		return nil, fmt.Errorf("cert expiring soon")
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		return nil, fmt.Errorf("cert not signed by current CA: %w", err)
	}

	cert.Leaf = leaf
	return &cert, nil
}

func generateServerCert(hostname string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) (*tls.Certificate, error) {
	return generateServerCertWithExpiry(hostname, caCert, caKey, 365*24*time.Hour)
}

func generateServerCertWithExpiry(hostname string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, validity time.Duration) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating server key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   hostname,
			Organization: []string{"Outport Dev CA"},
		},
		DNSNames:  []string{hostname},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(validity),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating server certificate: %w", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

func saveCertToDisk(cacheDir, hostname string, cert *tls.Certificate) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cert cache dir: %w", err)
	}

	certPath := filepath.Join(cacheDir, hostname+".pem")
	if err := writeCertPEM(certPath, cert.Certificate[0]); err != nil {
		return fmt.Errorf("encoding cert PEM: %w", err)
	}

	ecKey, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("private key is not ECDSA")
	}

	keyPath := filepath.Join(cacheDir, hostname+"-key.pem")
	if err := writeKeyPEM(keyPath, ecKey); err != nil {
		return fmt.Errorf("encoding key PEM: %w", err)
	}

	return nil
}

// DeleteCertCache removes the entire certificate cache directory
// (~/.cache/outport/certs/) and all server certificates within it. This is used
// during cleanup operations like "outport system prune". The certificates will be
// regenerated on demand by GetOrCreateCert when they are next needed, so this
// operation is safe to perform at any time.
func DeleteCertCache() error {
	dir, err := CertCacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

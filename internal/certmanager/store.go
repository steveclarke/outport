// internal/certmanager/store.go
package certmanager

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
)

// CertStore provides thread-safe certificate management with memory and disk caching.
// Use GetCertificate as a tls.Config.GetCertificate callback.
type CertStore struct {
	caCert   *x509.Certificate
	caKey    *ecdsa.PrivateKey
	cacheDir string

	mu    sync.RWMutex
	certs map[string]*tls.Certificate
}

// NewCertStore creates a CertStore backed by the given CA and cache directory.
func NewCertStore(caCertPath, caKeyPath, cacheDir string) (*CertStore, error) {
	caCert, caKey, err := LoadCA(caCertPath, caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading CA: %w", err)
	}
	return &CertStore{
		caCert:   caCert,
		caKey:    caKey,
		cacheDir: cacheDir,
		certs:    make(map[string]*tls.Certificate),
	}, nil
}

// GetCertificate implements tls.Config.GetCertificate.
// Checks memory cache, then disk cache, then generates a new cert.
func (s *CertStore) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	hostname := hello.ServerName

	// Check memory cache (read lock)
	s.mu.RLock()
	cert, ok := s.certs[hostname]
	s.mu.RUnlock()
	if ok {
		return cert, nil
	}

	// Acquire write lock with double-check
	s.mu.Lock()
	cert, ok = s.certs[hostname]
	if ok {
		s.mu.Unlock()
		return cert, nil
	}

	// Get or create (checks disk, generates if needed)
	cert, err := GetOrCreateCert(hostname, s.caCert, s.caKey, s.cacheDir)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("getting cert for %s: %w", hostname, err)
	}

	s.certs[hostname] = cert
	s.mu.Unlock()

	return cert, nil
}

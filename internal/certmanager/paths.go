// internal/certmanager/paths.go
package certmanager

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns ~/.local/share/outport/ (persistent, machine-specific data).
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "outport"), nil
}

// CACertPath returns the path to the CA certificate.
func CACertPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ca-cert.pem"), nil
}

// CAKeyPath returns the path to the CA private key.
func CAKeyPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ca-key.pem"), nil
}

// CertCacheDir returns ~/.cache/outport/certs/ (regenerable server certs).
func CertCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "outport", "certs"), nil
}

// IsCAInstalled returns true if the CA cert exists on disk.
func IsCAInstalled() bool {
	path, err := CACertPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

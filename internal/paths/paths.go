// Package paths provides functions that return the standard filesystem locations
// where Outport stores its data. These follow the XDG Base Directory Specification
// conventions on macOS: ~/.local/share for persistent data. The returned directories
// may not exist yet; callers are responsible for creating them as needed.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns the path to ~/.local/share/outport/, the directory for persistent,
// machine-specific data. This is where the registry.json file lives, which stores
// all project allocations (ports, hostnames, env vars) and instance identity. The
// directory may not exist on first run; callers should create it with os.MkdirAll.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "outport"), nil
}

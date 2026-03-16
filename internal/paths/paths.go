package paths

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

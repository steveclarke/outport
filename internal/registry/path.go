package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".config", "outport", "registry.json"), nil
}

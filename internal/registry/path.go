package registry

import (
	"path/filepath"

	"github.com/steveclarke/outport/internal/paths"
)

func DefaultPath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "registry.json"), nil
}

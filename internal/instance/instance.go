package instance

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/outport-app/outport/internal/registry"
)

const consonants = "bcdfghjkmnpqrstvwxz"

var validNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// GenerateCode generates a random 4-character code from consonants only.
// The used map is checked to avoid collisions.
func GenerateCode(used map[string]bool) string {
	for {
		code := randomCode(4)
		if !used[code] {
			return code
		}
	}
}

func randomCode(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(consonants))))
		b[i] = consonants[n.Int64()]
	}
	return string(b)
}

func isConsonant(c byte) bool {
	for i := 0; i < len(consonants); i++ {
		if consonants[i] == c {
			return true
		}
	}
	return false
}

// Resolve determines the instance name for a project in a given directory.
// Returns the instance name, whether it's newly created, and any error.
// NOTE: Resolve does NOT modify the registry. The caller is responsible for
// calling reg.Set() and reg.Save() to persist the new instance.
func Resolve(reg *registry.Registry, project, dir string) (string, bool, error) {
	// Check if this directory is already registered
	key, _, ok := reg.FindByDir(dir)
	if ok {
		parts := strings.SplitN(key, "/", 2)
		return parts[1], false, nil
	}

	// Check if any instance of this project exists
	existing := reg.FindByProject(project)
	if len(existing) == 0 {
		return "main", true, nil
	}

	// Generate a unique code
	usedNames := make(map[string]bool)
	for key := range existing {
		parts := strings.SplitN(key, "/", 2)
		usedNames[parts[1]] = true
	}
	code := GenerateCode(usedNames)
	return code, true, nil
}

// ValidateName validates instance names: lowercase alphanumeric and hyphens only,
// must start with alphanumeric.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("instance name %q must be lowercase alphanumeric and hyphens only", name)
	}
	return nil
}

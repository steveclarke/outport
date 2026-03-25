// Package instance handles the identity model for project checkouts. In Outport,
// a single project (defined by its outport.yml) can exist in multiple directories
// on disk — for example, when using git worktrees or multiple clones. Each
// directory gets its own "instance" identity that determines its registry key,
// port allocations, and hostname suffixes.
//
// The first checkout of a project is the "main" instance and gets clean hostnames
// (e.g., "myapp.test"). Additional checkouts receive auto-generated 4-character
// codes (e.g., "bxcf") and get suffixed hostnames (e.g., "myapp-bxcf.test").
// Instances can be renamed via "outport rename" or promoted to main via
// "outport promote".
//
// Instance codes use consonants only (no vowels) to avoid accidentally generating
// real words or offensive terms.
package instance

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/steveclarke/outport/internal/registry"
)

// consonants is the character set used to generate random instance codes.
// Vowels are deliberately excluded to prevent the codes from forming
// recognizable (and potentially offensive) words.
const consonants = "bcdfghjkmnpqrstvwxz"

// validNameRe defines the allowed format for instance names: must start with a
// lowercase letter or digit, followed by any combination of lowercase letters,
// digits, and hyphens. This is used by ValidateName to enforce naming rules
// for user-provided instance names (e.g., via "outport rename").
var validNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// GenerateCode generates a cryptographically random 4-character instance code
// using consonants only. The used map contains instance names that are already
// taken for the same project, and GenerateCode keeps generating until it finds
// one that is not in the map. With 19 consonants and 4 characters, there are
// 130,321 possible codes, so collisions are rare in practice.
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

// Resolve determines the instance name for a project located in the given
// directory. It implements the following logic:
//
//  1. If the directory is already registered (found via FindByDir), return its
//     existing instance name. This makes "outport up" idempotent.
//  2. If no instance of this project exists anywhere, this is the first checkout,
//     so it gets the "main" instance name.
//  3. If other instances exist but not for this directory, generate a new unique
//     4-character code to distinguish this checkout.
//
// Returns three values: the instance name (e.g., "main" or "bxcf"), a boolean
// indicating whether this is a newly created instance (true) or an existing one
// (false), and any error encountered.
//
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

// ValidateName checks whether a user-provided instance name meets the naming
// rules: it must be non-empty, start with a lowercase letter or digit, and
// contain only lowercase letters, digits, and hyphens. This is used by commands
// like "outport rename" where the user supplies a custom instance name. Returns
// nil if the name is valid, or a descriptive error if it violates the rules.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("instance name %q must be lowercase alphanumeric and hyphens only", name)
	}
	return nil
}

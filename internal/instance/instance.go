package instance

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
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

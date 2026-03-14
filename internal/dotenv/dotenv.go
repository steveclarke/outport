package dotenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Merge writes port values into the .env file at path.
// Variables declared in ports are always overwritten if they exist.
// Variables not in ports are preserved untouched.
// Comments and blank lines are preserved.
func Merge(path string, ports map[string]string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .env: %w", err)
	}

	written := make(map[string]bool)

	// Update existing lines in place
	for i, line := range lines {
		name := parseVarName(line)
		if name == "" {
			continue
		}
		if value, ok := ports[name]; ok {
			lines[i] = fmt.Sprintf("%s=%s", name, value)
			written[name] = true
		}
	}

	// Append any new variables not already in the file
	var newVars []string
	for name := range ports {
		if !written[name] {
			newVars = append(newVars, name)
		}
	}
	sort.Strings(newVars)

	if len(newVars) > 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		for _, name := range newVars {
			lines = append(lines, fmt.Sprintf("%s=%s", name, ports[name]))
		}
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

// parseVarName extracts the variable name from a line.
// Handles "VAR=value" and "export VAR=value".
// Returns "" for comments, blank lines, and lines without =.
func parseVarName(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Strip export prefix
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimPrefix(trimmed, "export ")
		trimmed = strings.TrimSpace(trimmed)
	}

	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

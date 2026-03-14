package dotenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const marker = " # managed by outport"

func Merge(path string, ports map[string]string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .env: %w", err)
	}

	existing := make(map[string]int)
	isManaged := make(map[string]bool)

	for i, line := range lines {
		name := parseVarName(line)
		if name != "" {
			existing[name] = i
			isManaged[name] = strings.Contains(line, marker)
		}
	}

	var newLines []string
	for varName, value := range ports {
		if idx, exists := existing[varName]; exists {
			if isManaged[varName] {
				lines[idx] = fmt.Sprintf("%s=%s%s", varName, value, marker)
			}
		} else {
			newLines = append(newLines, fmt.Sprintf("%s=%s%s", varName, value, marker))
		}
	}

	sort.Strings(newLines)

	if len(newLines) > 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, newLines...)
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

func parseVarName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	line = strings.Replace(line, marker, "", 1)
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

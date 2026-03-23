package dotenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	BeginMarker = "# --- begin outport.dev ---"
	EndMarker   = "# --- end outport.dev ---"
)

// Merge writes managed variables into a fenced block at the end of the .env file.
// Variables in the ports map are always written inside the managed block.
// Any matching variables in the user section (above/below the block) are removed.
// All other lines (comments, unrelated variables, blank lines) are preserved.
func Merge(path string, ports map[string]string) error {
	lines, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .env: %w", err)
	}

	if len(ports) == 0 {
		// Nothing to manage — remove any existing block and write back
		userLines, _, afterLines := splitBlock(lines)
		allLines := append(userLines, afterLines...)
		return writeLines(path, allLines)
	}

	// Split existing content around the managed block
	beforeLines, _, afterLines := splitBlock(lines)

	// Remove managed vars from user sections (before and after block)
	beforeLines = removeVars(beforeLines, ports)
	afterLines = removeVars(afterLines, ports)

	// Build the managed block content (sorted)
	var blockLines []string
	varNames := make([]string, 0, len(ports))
	for name := range ports {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)
	for _, name := range varNames {
		blockLines = append(blockLines, fmt.Sprintf("%s=%s", name, ports[name]))
	}

	// Assemble: before + blank separator + block + after
	var result []string
	result = append(result, beforeLines...)

	// Add blank line separator if there's content before the block
	if len(result) > 0 && result[len(result)-1] != "" {
		result = append(result, "")
	}

	result = append(result, BeginMarker)
	result = append(result, blockLines...)
	result = append(result, EndMarker)

	if len(afterLines) > 0 {
		result = append(result, afterLines...)
	}

	return writeLines(path, result)
}

// splitBlock separates lines into: before the block, inside the block, after the block.
// If no block exists, all lines are "before" and the other two are empty.
func splitBlock(lines []string) (before, block, after []string) {
	beginIdx := -1
	endIdx := -1

	for i, line := range lines {
		if line == BeginMarker {
			beginIdx = i
		}
		if line == EndMarker {
			endIdx = i
		}
	}

	if beginIdx == -1 || endIdx == -1 || endIdx <= beginIdx {
		return lines, nil, nil
	}

	before = lines[:beginIdx]
	block = lines[beginIdx+1 : endIdx]
	if endIdx+1 < len(lines) {
		after = lines[endIdx+1:]
	}
	return before, block, after
}

// removeVars removes lines from the slice whose variable name is in the ports map.
// Preserves comments, blank lines, and unrelated variables.
func removeVars(lines []string, ports map[string]string) []string {
	var result []string
	for _, line := range lines {
		name := parseVarName(line)
		if name != "" {
			if _, managed := ports[name]; managed {
				continue // remove this line
			}
		}
		result = append(result, line)
	}
	return result
}

// RemoveBlock removes the managed block from the file, leaving only user content.
func RemoveBlock(path string) error {
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading .env: %w", err)
	}

	before, _, after := splitBlock(lines)
	result := append(before, after...)

	// Trim trailing blank lines
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	return writeLines(path, result)
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	// Atomic write: temp file + rename to avoid partial writes on crash.
	// Matches the pattern used by registry.Save and tunnel.WriteState.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
// Handles "VAR=value", "VAR=value # comment", and "export VAR=value".
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

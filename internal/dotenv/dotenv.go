// Package dotenv manages the writing of environment variables into .env files
// using a fenced block pattern. Rather than owning the entire .env file, Outport
// writes its managed variables inside a clearly delimited section marked by
// begin/end comment lines. This allows developers to keep their own variables,
// comments, and blank lines in the same file without Outport touching them.
//
// The fenced block looks like this in a .env file:
//
//	MY_CUSTOM_VAR=something
//
//	# --- begin outport.dev ---
//	API_PORT=28901
//	WEB_PORT=13542
//	# --- end outport.dev ---
//
// If a developer manually adds a variable that Outport also manages (e.g., they
// write WEB_PORT=3000 above the block), the Merge function detects the conflict,
// removes the manual definition, and writes the Outport-managed value inside the
// block. This ensures Outport's deterministic port assignments always win while
// keeping the file clean.
//
// The package uses atomic writes (write to temp file, then rename) to prevent
// partial writes on crash, matching the pattern used by the registry and tunnel
// packages.
package dotenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// BeginMarker is the comment line that marks the start of the Outport-managed
	// section in a .env file. Everything between this line and EndMarker is owned
	// by Outport and will be overwritten on each "outport up" run.
	BeginMarker = "# --- begin outport.dev ---"

	// EndMarker is the comment line that marks the end of the Outport-managed
	// section. Content after this line belongs to the developer and is preserved.
	EndMarker = "# --- end outport.dev ---"
)

// Merge writes the given variables into the fenced Outport block of the .env file
// at the specified path. It is the primary function called during "outport up" to
// write computed environment variables (ports, URLs, etc.) into a project's .env files.
//
// The ports parameter maps variable names to their values (e.g., {"WEB_PORT": "13542"}).
// Despite the parameter name, these can be any env vars, not just ports.
//
// Merge handles several scenarios:
//   - If the file does not exist, it creates it with just the managed block.
//   - If the file exists but has no block, it appends the block at the end.
//   - If the file already has a block, it replaces the block contents.
//   - If the user section contains variables that Outport also manages, those
//     duplicates are removed from the user section (relocated into the block).
//   - If ports is empty, any existing block is removed and the file is cleaned up.
//
// Variables inside the block are sorted alphabetically for deterministic output.
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

// RemoveBlock removes the entire Outport-managed fenced section from the .env
// file at the given path, leaving only the developer's own content. This is
// called during "outport down" to clean up managed variables when a project is
// unregistered. If the file does not exist, RemoveBlock returns nil (no error).
// Trailing blank lines are trimmed after block removal to keep the file tidy.
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

// Package envpath classifies env file paths as internal or external relative to
// a project directory, and handles user approval for writing to external files.
//
// When an outport.yml config references env files outside the project directory
// (e.g., "../sibling/.env"), those writes could modify files the developer didn't
// intend to change. This package resolves all paths through symlinks to prevent
// symlink-based escapes, classifies each path as internal or external, and gates
// external writes behind an interactive approval prompt (or the -y flag).
//
// The approval workflow is: classify paths, check which external paths haven't
// been approved yet, prompt the user (or auto-approve), and persist the newly
// approved paths in the registry for future runs.
package envpath

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"
)

// ErrUserDenied is returned when the user explicitly answers "no" at the interactive
// approval prompt for external env file writes. Commands should treat this as a
// non-retryable error and exit without writing any env files.
var ErrUserDenied = errors.New("external env file write denied by user")

// ErrNonInteractive is returned when external env files need approval but stdin is
// not a terminal (e.g., running in CI or piped input). The error message suggests
// using the -y flag to auto-approve or moving the env files inside the project directory.
var ErrNonInteractive = errors.New("external env files require interactive approval; use -y to allow or move files inside the project directory")

// EnvFilePath holds a classified env file path. Each entry represents one env_file
// entry from the project's outport.yml, enriched with its resolved absolute path
// and a flag indicating whether it falls outside the project directory boundary.
type EnvFilePath struct {
	// ConfigPath is the path exactly as written in outport.yml (e.g., "../sibling/.env"
	// or ".env"). Used for display in prompts and error messages.
	ConfigPath string

	// ResolvedPath is the absolute filesystem path after resolving any symlinks.
	// This is the path used for all boundary checks and file operations, preventing
	// symlink-based escapes from the project directory.
	ResolvedPath string

	// External is true when ResolvedPath falls outside the project directory tree.
	// External paths require explicit user approval before Outport will write to them.
	External bool
}

// ClassifyEnvFiles resolves env file paths relative to projectDir and classifies
// each as internal (within the projectDir tree) or external (outside it). Both
// the projectDir and each env file path are resolved through symlinks before
// comparison, ensuring that a symlink pointing outside the project is correctly
// classified as external. Relative paths in envFiles are joined to projectDir
// before resolution; absolute paths are resolved directly.
func ClassifyEnvFiles(projectDir string, envFiles []string) ([]EnvFilePath, error) {
	realProjectDir, err := resolveDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolving project directory: %w", err)
	}
	prefix := realProjectDir + string(filepath.Separator)

	paths := make([]EnvFilePath, 0, len(envFiles))
	for _, envFile := range envFiles {
		var absPath string
		if filepath.IsAbs(envFile) {
			absPath = envFile
		} else {
			absPath = filepath.Join(projectDir, envFile)
		}

		resolved, err := resolveFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("resolving env file %q: %w", envFile, err)
		}

		external := resolved != realProjectDir && !strings.HasPrefix(resolved, prefix)
		paths = append(paths, EnvFilePath{
			ConfigPath:   envFile,
			ResolvedPath: resolved,
			External:     external,
		})
	}

	return paths, nil
}

// ExternalPaths filters a slice of classified paths to only those marked as external.
// Returns nil if no paths are external. This is a convenience function used by commands
// to quickly check whether any env files require approval before writing.
func ExternalPaths(paths []EnvFilePath) []EnvFilePath {
	var ext []EnvFilePath
	for _, p := range paths {
		if p.External {
			ext = append(ext, p)
		}
	}
	return ext
}

// ConfirmExternalFiles checks for unapproved external env file paths and handles
// the approval flow. It compares external paths against the list of previously
// approved paths (stored in the registry). For any unapproved paths, it either:
//   - auto-approves them when autoApprove is true (the -y flag)
//   - prompts the user interactively when stdin is a terminal
//   - returns ErrNonInteractive when stdin is not a terminal
//
// Returns the list of newly approved resolved paths, which the caller should persist
// to the registry so they are not prompted again on future runs. Returns nil when
// all external paths are already approved.
func ConfirmExternalFiles(
	paths []EnvFilePath,
	approvedPaths []string,
	projectDir string,
	autoApprove bool,
	stdin io.Reader,
	stderr io.Writer,
) ([]string, error) {
	approvedSet := make(map[string]bool, len(approvedPaths))
	for _, p := range approvedPaths {
		approvedSet[p] = true
	}

	var unapproved []EnvFilePath
	for _, p := range paths {
		if p.External && !approvedSet[p.ResolvedPath] {
			unapproved = append(unapproved, p)
		}
	}

	if len(unapproved) == 0 {
		return nil, nil
	}

	if !autoApprove {
		isInteractive := false
		if f, ok := stdin.(interface{ Fd() uintptr }); ok {
			isInteractive = term.IsTerminal(f.Fd())
		}

		if !isInteractive {
			return nil, ErrNonInteractive
		}

		fmt.Fprintf(stderr, "\n⚠ External env files detected:\n")
		for _, p := range unapproved {
			fmt.Fprintf(stderr, "  %s  →  %s\n", p.ConfigPath, p.ResolvedPath)
		}
		fmt.Fprintf(stderr, "\nThese files are outside the project directory (%s).\n", projectDir)
		fmt.Fprintf(stderr, "Allow writing to these files? [y/N] ")

		var response string
		_, _ = fmt.Fscanln(stdin, &response)

		if !strings.HasPrefix(strings.ToLower(response), "y") {
			return nil, ErrUserDenied
		}
	}

	newly := make([]string, len(unapproved))
	for i, p := range unapproved {
		newly[i] = p.ResolvedPath
	}
	return newly, nil
}

// resolveDir resolves a directory path through symlinks to its real absolute path.
func resolveDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveFile resolves a file path through symlinks. If the file doesn't exist yet,
// resolves the parent directory and joins the filename back.
func resolveFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}

	if !os.IsNotExist(err) {
		return "", err
	}

	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", fmt.Errorf("parent directory: %w", err)
	}
	return filepath.Join(parentResolved, filepath.Base(abs)), nil
}

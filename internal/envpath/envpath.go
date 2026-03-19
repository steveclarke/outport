package envpath

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"
)

// EnvFilePath holds a classified env file path.
type EnvFilePath struct {
	ConfigPath   string // as written in .outport.yml, e.g. "../sibling/.env"
	ResolvedPath string // absolute real path after symlink resolution
	External     bool   // true if outside the project directory tree
}

// ClassifyEnvFiles resolves env file paths relative to projectDir and classifies
// each as internal (within projectDir tree) or external (outside it).
// Both projectDir and env file paths are resolved through symlinks for accurate comparison.
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

// ExternalPaths filters classified paths to only those marked as external.
func ExternalPaths(paths []EnvFilePath) []EnvFilePath {
	var ext []EnvFilePath
	for _, p := range paths {
		if p.External {
			ext = append(ext, p)
		}
	}
	return ext
}

// ConfirmExternalFiles checks for unapproved external env file paths and either
// prompts for approval (interactive), auto-approves (-y flag), or returns an error
// (non-interactive without -y). Returns the list of newly approved resolved paths.
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

	if autoApprove {
		newly := make([]string, len(unapproved))
		for i, p := range unapproved {
			newly[i] = p.ResolvedPath
		}
		return newly, nil
	}

	isInteractive := false
	if f, ok := stdin.(interface{ Fd() uintptr }); ok {
		isInteractive = term.IsTerminal(f.Fd())
	}

	if !isInteractive {
		return nil, fmt.Errorf("external env files require interactive approval; use -y to allow or move files inside the project directory")
	}

	fmt.Fprintf(stderr, "\n⚠ External env files detected:\n")
	for _, p := range unapproved {
		fmt.Fprintf(stderr, "  %s  →  %s\n", p.ConfigPath, p.ResolvedPath)
	}
	fmt.Fprintf(stderr, "\nThese files are outside the project directory (%s).\n", projectDir)
	fmt.Fprintf(stderr, "Allow writing to these files? [y/N] ")

	var response string
	_, _ = fmt.Fscanln(stdin, &response)

	if response != "y" && response != "Y" {
		return nil, fmt.Errorf("external env file write denied by user")
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

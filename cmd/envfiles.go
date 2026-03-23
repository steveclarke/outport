package cmd

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocation"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/envpath"
	"github.com/outport-app/outport/internal/ui"
)

// handleConfirmError translates envpath confirmation errors into cmd-layer errors.
// User denial becomes ErrSilent (no redundant error message).
// Non-interactive errors become FlagErrors (trigger usage display).
func handleConfirmError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, envpath.ErrUserDenied) {
		return ErrSilent
	}
	if errors.Is(err, envpath.ErrNonInteractive) {
		return &FlagError{err: err}
	}
	return fmt.Errorf("confirming external env files: %w", err)
}

// EnvWriteOptions groups the approval and I/O parameters for env file operations.
type EnvWriteOptions struct {
	AutoApprove   bool
	ApprovedPaths []string
	TunnelURLs    map[string]string
	Stdin         io.Reader
	Stderr        io.Writer
}

// WriteResult bundles the results of writeEnvFiles.
type WriteResult struct {
	ResolvedComputed map[string]map[string]string
	ExternalFiles    []envpath.EnvFilePath
	NewlyApproved    []string
}

// RemoveResult bundles the results of removeEnvFiles.
type RemoveResult struct {
	CleanedFiles  []string // config-relative paths
	ExternalFiles []envpath.EnvFilePath
	NewlyApproved []string
}

// collectEnvFiles gathers all env file paths from config (services + computed).
func collectEnvFiles(cfg *config.Config) []string {
	seen := make(map[string]bool)
	for _, svc := range cfg.Services {
		for _, f := range svc.EnvFiles {
			seen[f] = true
		}
	}
	for _, dv := range cfg.Computed {
		for _, f := range dv.EnvFiles {
			seen[f] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// classifyAndConfirm collects env file paths from config, classifies them as
// internal/external, and confirms any unapproved external paths.
func classifyAndConfirm(
	dir string, cfg *config.Config,
	autoApprove bool, approvedPaths []string,
	stdin io.Reader, stderr io.Writer,
) ([]envpath.EnvFilePath, []string, error) {
	allFiles := collectEnvFiles(cfg)

	classified, err := envpath.ClassifyEnvFiles(dir, allFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("classifying env file paths: %w", err)
	}

	newlyApproved, err := envpath.ConfirmExternalFiles(classified, approvedPaths, dir, autoApprove, stdin, stderr)
	if err != nil {
		return nil, nil, handleConfirmError(err)
	}

	return classified, newlyApproved, nil
}

// writeEnvFiles classifies, confirms, and writes env files for an allocation.
func writeEnvFiles(
	dir string, cfg *config.Config, instanceName string,
	ports map[string]int, hostnames map[string]string,
	httpsEnabled bool, opts EnvWriteOptions,
) (*WriteResult, error) {
	classified, newlyApproved, err := classifyAndConfirm(dir, cfg, opts.AutoApprove, opts.ApprovedPaths, opts.Stdin, opts.Stderr)
	if err != nil {
		return nil, err
	}

	resolvedComputed, err := mergeEnvFiles(dir, cfg, instanceName, ports, hostnames, httpsEnabled, opts.TunnelURLs)
	if err != nil {
		return nil, fmt.Errorf("writing env files: %w", err)
	}

	return &WriteResult{
		ResolvedComputed: resolvedComputed,
		ExternalFiles:    envpath.ExternalPaths(classified),
		NewlyApproved:    newlyApproved,
	}, nil
}

// mergeEnvFiles rebuilds and writes env file vars for an allocation.
// Called by writeEnvFiles after external file confirmation.
// Returns the resolved computed values so callers can reuse them for display.
func mergeEnvFiles(dir string, cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) (map[string]map[string]string, error) {
	envFileVars := make(map[string]map[string]string)

	for svcName, svc := range cfg.Services {
		port := ports[svcName]
		for _, envFile := range svc.EnvFiles {
			if envFileVars[envFile] == nil {
				envFileVars[envFile] = make(map[string]string)
			}
			envFileVars[envFile][svc.EnvVar] = fmt.Sprintf("%d", port)
		}
	}

	// Resolve computed values and add to envFileVars
	resolvedComputed := allocation.ResolveComputed(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
	for name, fileValues := range resolvedComputed {
		for file, value := range fileValues {
			if envFileVars[file] == nil {
				envFileVars[file] = make(map[string]string)
			}
			envFileVars[file][name] = value
		}
	}

	envFiles := slices.Sorted(maps.Keys(envFileVars))
	for _, envFile := range envFiles {
		envPath := filepath.Join(dir, envFile)
		if err := dotenv.Merge(envPath, envFileVars[envFile]); err != nil {
			return nil, fmt.Errorf("writing %s: %w", envFile, err)
		}
	}

	return resolvedComputed, nil
}

// cleanEnvFiles removes the outport fenced block from all .env files
// referenced by the config. Returns the list of files that were cleaned.
func cleanEnvFiles(dir string, cfg *config.Config) []string {
	var cleaned []string
	for _, f := range collectEnvFiles(cfg) {
		if err := dotenv.RemoveBlock(filepath.Join(dir, f)); err == nil {
			cleaned = append(cleaned, f)
		}
	}
	return cleaned
}

// removeEnvFiles classifies, confirms, and removes the outport fenced block from env files.
func removeEnvFiles(dir string, cfg *config.Config, opts EnvWriteOptions) (*RemoveResult, error) {
	classified, newlyApproved, err := classifyAndConfirm(dir, cfg, opts.AutoApprove, opts.ApprovedPaths, opts.Stdin, opts.Stderr)
	if err != nil {
		return nil, err
	}

	cleanedFiles := cleanEnvFiles(dir, cfg)

	return &RemoveResult{
		CleanedFiles:  cleanedFiles,
		ExternalFiles: envpath.ExternalPaths(classified),
		NewlyApproved: newlyApproved,
	}, nil
}

// mergeApprovedPaths merges two slices of approved paths, deduplicating by value.
func mergeApprovedPaths(existing, newly []string) []string {
	seen := make(map[string]bool, len(existing)+len(newly))
	var merged []string
	for _, p := range existing {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	for _, p := range newly {
		if !seen[p] {
			seen[p] = true
			merged = append(merged, p)
		}
	}
	return merged
}

// printExternalFilesWarning prints a warning about env files written outside the project directory.
func printExternalFilesWarning(w io.Writer, external []envpath.EnvFilePath) {
	if len(external) == 0 {
		return
	}
	lipgloss.Fprintln(w)
	warnStyle := lipgloss.NewStyle().Foreground(ui.Yellow)
	lipgloss.Fprintln(w, warnStyle.Render("⚠ Note: env files written outside the project directory:"))
	for _, f := range external {
		lipgloss.Fprintln(w, warnStyle.Render(fmt.Sprintf("  %s  →  %s", f.ConfigPath, f.ResolvedPath)))
	}
}

type externalFileJSON struct {
	ConfigPath   string `json:"config_path"`
	ResolvedPath string `json:"resolved_path"`
}

func toExternalFileJSON(paths []envpath.EnvFilePath) []externalFileJSON {
	if len(paths) == 0 {
		return nil
	}
	result := make([]externalFileJSON, len(paths))
	for i, p := range paths {
		result[i] = externalFileJSON{
			ConfigPath:   p.ConfigPath,
			ResolvedPath: p.ResolvedPath,
		}
	}
	return result
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/envpath"
	"github.com/outport-app/outport/internal/instance"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:     "rename <old> <new>",
	Short:   "Rename an instance of the current project",
	Long:    "Renames an instance in the registry and updates hostnames in .env files.",
	GroupID: "project",
	Args:    ExactArgs(2, "requires two arguments: outport rename <old-name> <new-name>"),
	RunE:    runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	if oldName == newName {
		return fmt.Errorf("old and new instance names are the same")
	}

	if err := instance.ValidateName(newName); err != nil {
		return fmt.Errorf("invalid new name: %w", err)
	}

	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	// Check old instance exists
	oldAlloc, ok := reg.Get(cfg.Name, oldName)
	if !ok {
		return fmt.Errorf("instance %q not found for project %q", oldName, cfg.Name)
	}

	// Check new instance name doesn't collide
	if _, exists := reg.Get(cfg.Name, newName); exists {
		return fmt.Errorf("instance %q already exists for project %q", newName, cfg.Name)
	}

	// Move the allocation: delete old key, recompute hostnames, set new key
	reg.Remove(cfg.Name, oldName)

	newAlloc := buildAllocation(cfg, newName, oldAlloc.ProjectDir, oldAlloc.Ports)
	newAlloc.ApprovedExternalFiles = oldAlloc.ApprovedExternalFiles
	reg.Set(cfg.Name, newName, newAlloc)

	// Re-merge .env files with updated hostnames
	httpsEnabled := certmanager.IsCAInstalled()
	result, err := writeEnvFiles(ctx.Dir, cfg, newName, oldAlloc.Ports, newAlloc.Hostnames, httpsEnabled, nil,
		yesFlag, newAlloc.ApprovedExternalFiles, os.Stdin, os.Stderr)
	if err != nil {
		return fmt.Errorf("updating .env files: %w", err)
	}

	if len(result.NewlyApproved) > 0 {
		newAlloc.ApprovedExternalFiles = mergeApprovedPaths(newAlloc.ApprovedExternalFiles, result.NewlyApproved)
		reg.Set(cfg.Name, newName, newAlloc)
	}

	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printRenameJSON(cmd, cfg.Name, oldName, newName, result.ExternalFiles)
	}
	err = printRenameStyled(cmd, cfg.Name, oldName, newName)
	if err != nil {
		return err
	}
	printExternalFilesWarning(cmd.OutOrStdout(), result.ExternalFiles)
	return nil
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
	resolvedComputed := resolveComputedFromAlloc(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
	for name, fileValues := range resolvedComputed {
		for file, value := range fileValues {
			if envFileVars[file] == nil {
				envFileVars[file] = make(map[string]string)
			}
			envFileVars[file][name] = value
		}
	}

	envFiles := sortedMapKeys(envFileVars)
	for _, envFile := range envFiles {
		envPath := filepath.Join(dir, envFile)
		if err := dotenv.Merge(envPath, envFileVars[envFile]); err != nil {
			return nil, fmt.Errorf("writing %s: %w", envFile, err)
		}
	}

	return resolvedComputed, nil
}

func printRenameJSON(cmd *cobra.Command, project, oldName, newName string, externalFiles []envpath.EnvFilePath) error {
	out := struct {
		Project       string             `json:"project"`
		OldInstance   string             `json:"old_instance"`
		NewInstance   string             `json:"new_instance"`
		Status        string             `json:"status"`
		ExternalFiles []externalFileJSON `json:"external_files,omitempty"`
	}{
		Project:       project,
		OldInstance:   oldName,
		NewInstance:   newName,
		Status:        "renamed",
		ExternalFiles: toExternalFileJSON(externalFiles),
	}
	return writeJSON(cmd, out)
}

func printRenameStyled(cmd *cobra.Command, project, oldName, newName string) error {
	w := cmd.OutOrStdout()
	printHeader(w, project, newName)
	msg := fmt.Sprintf("Renamed %s to %s.",
		ui.InstanceStyle.Render(oldName),
		ui.InstanceStyle.Render(newName))
	fmt.Fprintln(w, ui.SuccessStyle.Render(msg))
	return nil
}

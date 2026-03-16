package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/instance"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename an instance of the current project",
	Long:  "Renames an instance in the registry and updates hostnames in .env files.",
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
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
	reg.Set(cfg.Name, newName, newAlloc)

	// Re-merge .env files with updated hostnames
	if err := mergeEnvFiles(ctx.Dir, cfg, oldAlloc.Ports, newAlloc.Hostnames); err != nil {
		return fmt.Errorf("updating .env files: %w", err)
	}

	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printRenameJSON(cmd, cfg.Name, oldName, newName)
	}
	return printRenameStyled(cmd, cfg.Name, oldName, newName)
}

// mergeEnvFiles rebuilds and writes env file vars for an allocation.
// This is used by rename and promote to update .env files after hostnames change.
func mergeEnvFiles(dir string, cfg *config.Config, ports map[string]int, hostnames map[string]string) error {
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

	// Resolve derived values and add to envFileVars
	resolvedDerived := resolveDerivedFromAlloc(cfg, ports, hostnames)
	for name, fileValues := range resolvedDerived {
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
			return fmt.Errorf("writing %s: %w", envFile, err)
		}
	}

	return nil
}

func printRenameJSON(cmd *cobra.Command, project, oldName, newName string) error {
	out := struct {
		Project     string `json:"project"`
		OldInstance string `json:"old_instance"`
		NewInstance string `json:"new_instance"`
		Status      string `json:"status"`
	}{
		Project:     project,
		OldInstance: oldName,
		NewInstance: newName,
		Status:      "renamed",
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
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

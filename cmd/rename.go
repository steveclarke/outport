package cmd

import (
	"fmt"
	"os"

	"github.com/steveclarke/outport/internal/allocation"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/envpath"
	"github.com/steveclarke/outport/internal/instance"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:     "rename [old] <new>",
	Short:   "Rename an instance of the current project",
	Long:    "Renames an instance in the registry and updates hostnames in .env files.\nIf only one argument is given, renames the current directory's instance.",
	GroupID: "project",
	Args:    RangeArgs(1, 2, "requires at least one argument: outport rename <new-name>"),
	RunE:    runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	var oldName, newName string
	if len(args) == 2 {
		oldName = args[0]
		newName = args[1]
	} else {
		oldName = ctx.Instance
		newName = args[0]
	}

	if oldName == newName {
		return fmt.Errorf("old and new instance names are the same")
	}

	if err := instance.ValidateName(newName); err != nil {
		return fmt.Errorf("invalid new name: %w", err)
	}

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

	newAlloc := allocation.Build(cfg, newName, oldAlloc.ProjectDir, oldAlloc.Ports)
	newAlloc.ApprovedExternalFiles = oldAlloc.ApprovedExternalFiles
	reg.Set(cfg.Name, newName, newAlloc)

	// Re-merge .env files with updated hostnames
	httpsEnabled := certmanager.IsCAInstalled()
	result, err := writeEnvFiles(ctx.Dir, cfg, newName, oldAlloc.Ports, newAlloc.Hostnames, httpsEnabled, EnvWriteOptions{
		AutoApprove:   yesFlag,
		ApprovedPaths: newAlloc.ApprovedExternalFiles,
		Aliases:       newAlloc.Aliases,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
	})
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
	return writeJSON(cmd, out, fmt.Sprintf("instance renamed to %s", newName))
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

package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/envpath"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:     "down",
	Short:   "Remove this project from outport",
	Long:    "Removes the managed block from all .env files and removes the project from the central registry.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	alloc, ok := reg.Get(cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("project %q (instance %q) is not registered", cfg.Name, ctx.Instance)
	}

	// Clean managed blocks from .env files
	result, err := removeEnvFiles(ctx.Dir, cfg, EnvWriteOptions{
		AutoApprove:   yesFlag,
		ApprovedPaths: alloc.ApprovedExternalFiles,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
	})
	if err != nil {
		return err
	}

	reg.Remove(cfg.Name, ctx.Instance)
	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printDownJSON(cmd, cfg.Name, ctx.Instance, result.CleanedFiles, result.ExternalFiles)
	}
	err = printDownStyled(cmd, cfg.Name, ctx.Instance, result.CleanedFiles)
	if err != nil {
		return err
	}
	printExternalFilesWarning(cmd.OutOrStdout(), result.ExternalFiles)
	return nil
}

func printDownJSON(cmd *cobra.Command, project, instance string, cleanedFiles []string, externalFiles []envpath.EnvFilePath) error {
	out := struct {
		Project       string             `json:"project"`
		Instance      string             `json:"instance"`
		Status        string             `json:"status"`
		CleanedFiles  []string           `json:"cleaned_files"`
		ExternalFiles []externalFileJSON `json:"external_files,omitempty"`
	}{
		Project:       project,
		Instance:      instance,
		Status:        "removed",
		CleanedFiles:  cleanedFiles,
		ExternalFiles: toExternalFileJSON(externalFiles),
	}
	return writeJSON(cmd, out)
}

func printDownStyled(cmd *cobra.Command, project, instanceName string, cleanedFiles []string) error {
	w := cmd.OutOrStdout()
	printHeader(w, project, instanceName)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Done. All ports freed."))
	if len(cleanedFiles) > 0 {
		fmt.Fprintln(w, ui.SuccessStyle.Render("Cleaned managed variables from:"))
		for _, f := range cleanedFiles {
			fmt.Fprintln(w, ui.SuccessStyle.Render("  "+f))
		}
	}
	return nil
}

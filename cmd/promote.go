package cmd

import (
	"fmt"
	"os"

	"github.com/steveclarke/outport/internal/allocation"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/envpath"
	"github.com/steveclarke/outport/internal/instance"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:     "promote",
	Short:   "Promote the current instance to main",
	Long:    "Promotes the current worktree instance to \"main\", demoting the existing main instance if one exists.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runPromote,
}

func init() {
	rootCmd.AddCommand(promoteCmd)
}

func runPromote(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	if ctx.Instance == "main" {
		return fmt.Errorf("current instance is already \"main\"")
	}

	// Check current instance exists in registry
	currentAlloc, ok := reg.Get(cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("instance %q not found for project %q", ctx.Instance, cfg.Name)
	}

	httpsEnabled := certmanager.IsCAInstalled()
	var demotedTo string
	var allExternalFiles []envpath.EnvFilePath

	// If a "main" instance exists, demote it
	if mainAlloc, hasMain := reg.Get(cfg.Name, "main"); hasMain {
		// Generate a new code for the demoted main
		usedNames := make(map[string]bool)
		existing := reg.FindByProject(cfg.Name)
		for key := range existing {
			_, inst := registry.ParseKey(key)
			usedNames[inst] = true
		}
		demotedTo = instance.GenerateCode(usedNames)

		// Rekey main → generated code
		reg.Remove(cfg.Name, "main")

		demotedAlloc := allocation.Build(cfg, demotedTo, mainAlloc.ProjectDir, mainAlloc.Ports)
		demotedAlloc.ApprovedExternalFiles = mainAlloc.ApprovedExternalFiles
		reg.Set(cfg.Name, demotedTo, demotedAlloc)

		// Re-merge .env files for the demoted instance
		demotedResult, err := writeEnvFiles(mainAlloc.ProjectDir, cfg, demotedTo, mainAlloc.Ports, demotedAlloc.Hostnames, httpsEnabled, EnvWriteOptions{
			AutoApprove:   yesFlag,
			ApprovedPaths: demotedAlloc.ApprovedExternalFiles,
			Aliases:       demotedAlloc.Aliases,
			Stdin:         os.Stdin,
			Stderr:        os.Stderr,
		})
		if err != nil {
			return fmt.Errorf("updating .env files for demoted instance: %w", err)
		}

		if len(demotedResult.NewlyApproved) > 0 {
			demotedAlloc.ApprovedExternalFiles = mergeApprovedPaths(demotedAlloc.ApprovedExternalFiles, demotedResult.NewlyApproved)
			reg.Set(cfg.Name, demotedTo, demotedAlloc)
		}

		allExternalFiles = append(allExternalFiles, demotedResult.ExternalFiles...)
	}

	// Promote current instance → main
	reg.Remove(cfg.Name, ctx.Instance)

	promotedAlloc := allocation.Build(cfg, "main", currentAlloc.ProjectDir, currentAlloc.Ports)
	promotedAlloc.ApprovedExternalFiles = currentAlloc.ApprovedExternalFiles
	reg.Set(cfg.Name, "main", promotedAlloc)

	// Re-merge .env files for the promoted instance
	promotedResult, err := writeEnvFiles(ctx.Dir, cfg, "main", currentAlloc.Ports, promotedAlloc.Hostnames, httpsEnabled, EnvWriteOptions{
		AutoApprove:   yesFlag,
		ApprovedPaths: promotedAlloc.ApprovedExternalFiles,
		Aliases:       promotedAlloc.Aliases,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("updating .env files for promoted instance: %w", err)
	}

	if len(promotedResult.NewlyApproved) > 0 {
		promotedAlloc.ApprovedExternalFiles = mergeApprovedPaths(promotedAlloc.ApprovedExternalFiles, promotedResult.NewlyApproved)
		reg.Set(cfg.Name, "main", promotedAlloc)
	}

	allExternalFiles = append(allExternalFiles, promotedResult.ExternalFiles...)

	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printPromoteJSON(cmd, cfg.Name, ctx.Instance, demotedTo, allExternalFiles)
	}
	err = printPromoteStyled(cmd, cfg.Name, ctx.Instance, demotedTo)
	if err != nil {
		return err
	}
	printExternalFilesWarning(cmd.OutOrStdout(), allExternalFiles)
	return nil
}

func printPromoteJSON(cmd *cobra.Command, project, promoted, demotedTo string, externalFiles []envpath.EnvFilePath) error {
	out := struct {
		Project       string             `json:"project"`
		Promoted      string             `json:"promoted"`
		DemotedTo     string             `json:"demoted_to,omitempty"`
		Status        string             `json:"status"`
		ExternalFiles []externalFileJSON `json:"external_files,omitempty"`
	}{
		Project:       project,
		Promoted:      promoted,
		DemotedTo:     demotedTo,
		Status:        "promoted",
		ExternalFiles: toExternalFileJSON(externalFiles),
	}
	return writeJSON(cmd, out, "instance promoted to main")
}

func printPromoteStyled(cmd *cobra.Command, project, promoted, demotedTo string) error {
	w := cmd.OutOrStdout()
	printHeader(w, project, "main")
	msg := fmt.Sprintf("Promoted %s to main.", ui.InstanceStyle.Render(promoted))
	fmt.Fprintln(w, ui.SuccessStyle.Render(msg))
	if demotedTo != "" {
		msg := fmt.Sprintf("Previous main demoted to %s.", ui.InstanceStyle.Render(demotedTo))
		fmt.Fprintln(w, ui.SuccessStyle.Render(msg))
	}
	return nil
}

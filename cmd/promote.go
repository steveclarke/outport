package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/instance"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote the current instance to main",
	Long:  "Promotes the current worktree instance to \"main\", demoting the existing main instance if one exists.",
	RunE:  runPromote,
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

	useHTTPS = certmanager.IsCAInstalled()
	var demotedTo string

	// If a "main" instance exists, demote it
	if mainAlloc, hasMain := reg.Get(cfg.Name, "main"); hasMain {
		// Generate a new code for the demoted main
		usedNames := make(map[string]bool)
		existing := reg.FindByProject(cfg.Name)
		for key := range existing {
			parts := strings.SplitN(key, "/", 2)
			usedNames[parts[1]] = true
		}
		demotedTo = instance.GenerateCode(usedNames)

		// Rekey main → generated code
		reg.Remove(cfg.Name, "main")

		demotedAlloc := buildAllocation(cfg, demotedTo, mainAlloc.ProjectDir, mainAlloc.Ports)
		reg.Set(cfg.Name, demotedTo, demotedAlloc)

		// Re-merge .env files for the demoted instance
		if err := mergeEnvFiles(mainAlloc.ProjectDir, cfg, mainAlloc.Ports, demotedAlloc.Hostnames); err != nil {
			return fmt.Errorf("updating .env files for demoted instance: %w", err)
		}
	}

	// Promote current instance → main
	reg.Remove(cfg.Name, ctx.Instance)

	promotedAlloc := buildAllocation(cfg, "main", currentAlloc.ProjectDir, currentAlloc.Ports)
	reg.Set(cfg.Name, "main", promotedAlloc)

	// Re-merge .env files for the promoted instance
	if err := mergeEnvFiles(ctx.Dir, cfg, currentAlloc.Ports, promotedAlloc.Hostnames); err != nil {
		return fmt.Errorf("updating .env files for promoted instance: %w", err)
	}

	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printPromoteJSON(cmd, cfg.Name, ctx.Instance, demotedTo)
	}
	return printPromoteStyled(cmd, cfg.Name, ctx.Instance, demotedTo)
}

func printPromoteJSON(cmd *cobra.Command, project, promoted, demotedTo string) error {
	out := struct {
		Project   string `json:"project"`
		Promoted  string `json:"promoted"`
		DemotedTo string `json:"demoted_to,omitempty"`
		Status    string `json:"status"`
	}{
		Project:   project,
		Promoted:  promoted,
		DemotedTo: demotedTo,
		Status:    "promoted",
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
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

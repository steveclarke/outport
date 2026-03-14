package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var unregisterCmd = &cobra.Command{
	Use:     "unregister",
	Aliases: []string{"unreg"},
	Short:   "Remove project from the registry and free its ports",
	Long:    "Removes the current project/instance from the central registry, freeing all its port allocations.",
	RunE:    runUnregister,
}

func init() {
	rootCmd.AddCommand(unregisterCmd)
}

func runUnregister(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, wt, reg := ctx.Cfg, ctx.WT, ctx.Reg

	_, ok := reg.Get(cfg.Name, wt.Instance)
	if !ok {
		return fmt.Errorf("project %q (instance %q) is not registered", cfg.Name, wt.Instance)
	}

	reg.Remove(cfg.Name, wt.Instance)
	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printUnregisterJSON(cmd, cfg.Name, wt.Instance)
	}
	return printUnregisterStyled(cmd, cfg.Name, wt)
}

func printUnregisterJSON(cmd *cobra.Command, project, instance string) error {
	out := struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Status   string `json:"status"`
	}{
		Project:  project,
		Instance: instance,
		Status:   "unregistered",
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printUnregisterStyled(cmd *cobra.Command, project string, wt *worktree.Info) error {
	w := cmd.OutOrStdout()
	printHeader(w, project, wt)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Unregistered. All ports freed."))
	return nil
}

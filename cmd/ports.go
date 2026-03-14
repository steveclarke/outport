package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show ports for the current project",
	RunE:  runPorts,
}

func init() {
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(dir)
	if err != nil {
		return err
	}

	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	alloc, ok := reg.Get(cfg.Name, wt.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	if jsonFlag {
		return printPortsJSON(cmd, cfg, wt, alloc)
	}
	return printPortsStyled(cmd, cfg, wt, alloc)
}

func printPortsJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, alloc registry.Allocation) error {
	out := struct {
		Project  string         `json:"project"`
		Instance string         `json:"instance"`
		Services map[string]int `json:"services"`
	}{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: alloc.Ports,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printPortsStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, alloc registry.Allocation) error {
	w := cmd.OutOrStdout()

	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}

	header := ui.ProjectStyle.Render(cfg.Name) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)

	svcNames := make([]string, 0, len(alloc.Ports))
	for s := range alloc.Ports {
		svcNames = append(svcNames, s)
	}
	sort.Strings(svcNames)

	for _, svcName := range svcNames {
		port := alloc.Ports[svcName]
		envVar := svcName
		if svc, ok := cfg.Services[svcName]; ok {
			envVar = svc.EnvVar
		}
		line := fmt.Sprintf("  %s  %s  %s %s",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
			ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", envVar)),
			ui.Arrow,
			ui.PortStyle.Render(fmt.Sprintf("%d", port)),
		)
		lipgloss.Fprintln(w, line)
	}

	return nil
}

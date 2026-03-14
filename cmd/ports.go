package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
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
	out := upJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: buildServiceMap(cfg, alloc.Ports),
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

	printHeader(w, cfg.Name, wt)

	serviceNames := sortedMapKeys(alloc.Ports)

	hasGroups := false
	for _, svcName := range serviceNames {
		if svc, ok := cfg.Services[svcName]; ok && svc.Group != "" {
			hasGroups = true
			break
		}
	}

	if hasGroups {
		printGroupedServices(w, cfg, serviceNames, alloc.Ports)
	} else {
		printFlatServices(w, cfg, serviceNames, alloc.Ports)
	}

	return nil
}

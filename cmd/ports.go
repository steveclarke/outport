package cmd

import (
	"fmt"
	"os"
	"sort"

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

	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n", cfg.Name, instance)

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
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) → %d\n", svcName, envVar, port)
	}

	return nil
}

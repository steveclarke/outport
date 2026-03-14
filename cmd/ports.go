package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var portsCheckFlag bool

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show ports for the current project",
	RunE:  runPorts,
}

func init() {
	portsCmd.Flags().BoolVar(&portsCheckFlag, "check", false, "check if ports are accepting connections")
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.WT.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	if jsonFlag {
		return printPortsJSON(cmd, ctx.Cfg, ctx.WT, alloc)
	}
	return printPortsStyled(cmd, ctx.Cfg, ctx.WT, alloc)
}

func printPortsJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, alloc registry.Allocation) error {
	services := buildServiceMap(cfg, alloc.Ports)

	if portsCheckFlag {
		portStatus := checkPorts(alloc.Ports)
		for name, s := range services {
			s.Up = boolPtr(portStatus[s.Port])
			services[name] = s
		}
	}

	out := upJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: services,
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

	var portStatus map[int]bool
	if portsCheckFlag {
		portStatus = checkPorts(alloc.Ports)
	}

	if hasGroups(cfg, serviceNames) {
		printGroupedServices(w, cfg, serviceNames, alloc.Ports, portStatus)
	} else {
		printFlatServices(w, cfg, serviceNames, alloc.Ports, portStatus)
	}

	return nil
}

// checkPorts collects all port values and checks them concurrently.
func checkPorts(ports map[string]int) map[int]bool {
	var allPorts []int
	for _, port := range ports {
		allPorts = append(allPorts, port)
	}
	return portcheck.CheckAll(allPorts)
}

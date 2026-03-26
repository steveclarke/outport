package cmd

import (
	"fmt"
	"maps"
	"slices"

	"github.com/steveclarke/outport/internal/allocation"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/portcheck"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/spf13/cobra"
)

var portsCheckFlag bool
var portsComputedFlag bool

var portsCmd = &cobra.Command{
	Use:     "ports",
	Short:   "Show ports for the current project",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runPorts,
}

func init() {
	portsCmd.Flags().BoolVar(&portsCheckFlag, "check", false, "check if ports are accepting connections")
	portsCmd.Flags().BoolVar(&portsComputedFlag, "computed", false, "show computed values")
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsJSON(cmd, ctx.Cfg, ctx.Instance, alloc, httpsEnabled)
	}
	return printPortsStyled(cmd, ctx.Cfg, ctx.Instance, alloc, httpsEnabled)
}

func printPortsJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, httpsEnabled bool) error {
	services := buildServiceMap(cfg, alloc.Ports, alloc.Hostnames, httpsEnabled)

	if portsCheckFlag {
		portStatus := checkPorts(alloc.Ports)
		for name, s := range services {
			s.Up = boolPtr(portStatus[s.Port])
			services[name] = s
		}
	}

	out := upJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Services: services,
	}
	if portsComputedFlag {
		out.Computed = buildComputedMap(cfg.Computed, allocation.ResolveComputed(cfg, instanceName, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, nil))
	}
	return writeJSON(cmd, out)
}

func printPortsStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, instanceName)

	serviceNames := slices.Sorted(maps.Keys(alloc.Ports))

	var portStatus map[int]bool
	if portsCheckFlag {
		portStatus = checkPorts(alloc.Ports)
	}

	printFlatServices(w, cfg, serviceNames, alloc.Ports, alloc.Hostnames, portStatus, httpsEnabled)

	if portsComputedFlag {
		if resolved := allocation.ResolveComputed(cfg, instanceName, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, nil); len(resolved) > 0 {
			printComputedValues(w, resolved)
		}
	}

	return nil
}

// checkPorts collects all port values and checks them concurrently.
func checkPorts(ports map[string]int) map[int]bool {
	return portcheck.CheckAll(slices.Collect(maps.Values(ports)))
}

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

var projectStatusComputedFlag bool

var projectStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show status for the current project",
	Long:    "Shows ports, hostnames, health status, and computed values for the current project. Health checks run by default.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runProjectStatus,
}

func init() {
	projectStatusCmd.Flags().BoolVar(&projectStatusComputedFlag, "computed", false, "show computed values")
	rootCmd.AddCommand(projectStatusCmd)
}

func runProjectStatus(cmd *cobra.Command, args []string) error {
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
	portStatus := checkPorts(alloc.Ports)

	if jsonFlag {
		return printProjectStatusJSON(cmd, ctx.Cfg, ctx.Instance, alloc, portStatus, httpsEnabled)
	}
	return printProjectStatusStyled(cmd, ctx.Cfg, ctx.Instance, alloc, portStatus, httpsEnabled)
}

func printProjectStatusJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, portStatus map[int]bool, httpsEnabled bool) error {
	services := buildServiceMap(cfg, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled)

	for name, s := range services {
		s.Up = boolPtr(portStatus[s.Port])
		services[name] = s
	}

	out := upJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Services: services,
	}
	if projectStatusComputedFlag {
		out.Computed = buildComputedMap(cfg.Computed, allocation.ResolveComputed(cfg, instanceName, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, nil))
	}
	n := len(out.Services)
	summary := fmt.Sprintf("%d %s", n, pluralize(n, "service", "services"))
	return writeJSON(cmd, out, summary)
}

func printProjectStatusStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, portStatus map[int]bool, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, instanceName)

	serviceNames := slices.Sorted(maps.Keys(alloc.Ports))
	printFlatServices(w, cfg, serviceNames, alloc.Ports, alloc.Hostnames, alloc.Aliases, portStatus, httpsEnabled)

	if projectStatusComputedFlag {
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

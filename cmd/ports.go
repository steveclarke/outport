package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
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

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport apply' first.")
		return nil
	}

	if jsonFlag {
		return printPortsJSON(cmd, ctx.Cfg, ctx.Instance, alloc)
	}
	return printPortsStyled(cmd, ctx.Cfg, ctx.Instance, alloc)
}

func printPortsJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation) error {
	services := buildServiceMap(cfg, alloc.Ports, alloc.Hostnames)

	if portsCheckFlag {
		portStatus := checkPorts(alloc.Ports)
		for name, s := range services {
			s.Up = boolPtr(portStatus[s.Port])
			services[name] = s
		}
	}

	out := applyJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Services: services,
		Derived:  buildDerivedMap(cfg.Derived, resolveDerivedFromAlloc(cfg, alloc.Ports, alloc.Hostnames)),
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printPortsStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, instanceName)

	serviceNames := sortedMapKeys(alloc.Ports)

	var portStatus map[int]bool
	if portsCheckFlag {
		portStatus = checkPorts(alloc.Ports)
	}

	printFlatServices(w, cfg, serviceNames, alloc.Ports, alloc.Hostnames, portStatus)

	if resolved := resolveDerivedFromAlloc(cfg, alloc.Ports, alloc.Hostnames); len(resolved) > 0 {
		printDerivedValues(w, resolved)
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

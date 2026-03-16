package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [service]",
	Short: "Open HTTP services in the browser",
	Long:  "Opens all HTTP/HTTPS services for the current project in your default browser. Specify a service name to open just one.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("No ports allocated. Run 'outport apply' first.")
	}

	if len(args) == 1 {
		return openService(cmd, ctx.Cfg, alloc, args[0])
	}

	opened := 0
	for _, svcName := range sortedMapKeys(ctx.Cfg.Services) {
		svc := ctx.Cfg.Services[svcName]
		var url string
		if h, ok := alloc.Hostnames[svcName]; ok {
			protocol := svc.Protocol
			if protocol == "" {
				continue
			}
			url = fmt.Sprintf("%s://%s", protocol, h)
		} else {
			url = serviceURL(svc.Protocol, svc.Hostname, alloc.Ports[svcName], false)
		}
		if url == "" {
			continue
		}
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open %s: %v\n", svcName, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", svcName, url)
		opened++
	}

	if opened == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No HTTP services found. Add 'protocol: http' to services in .outport.yml.")
	}

	return nil
}

func openService(cmd *cobra.Command, cfg *config.Config, alloc registry.Allocation, name string) error {
	svc, ok := cfg.Services[name]
	if !ok {
		return fmt.Errorf("Service %q not found in .outport.yml.", name)
	}

	port, ok := alloc.Ports[name]
	if !ok {
		return fmt.Errorf("No port allocated for %q. Run 'outport apply' first.", name)
	}

	var url string
	if h, hok := alloc.Hostnames[name]; hok {
		if svc.Protocol == "" {
			return fmt.Errorf("Service %q has no protocol set. Add 'protocol: http' to open it in the browser.", name)
		}
		url = fmt.Sprintf("%s://%s", svc.Protocol, h)
	} else {
		url = serviceURL(svc.Protocol, svc.Hostname, port, false)
	}
	if url == "" {
		return fmt.Errorf("Service %q has no protocol set. Add 'protocol: http' to open it in the browser.", name)
	}

	if err := openBrowser(url); err != nil {
		return fmt.Errorf("Could not open browser: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", name, url)
	return nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("Unsupported platform %s.", runtime.GOOS)
	}
}

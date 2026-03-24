package cmd

import (
	"fmt"
	"maps"
	"os/exec"
	"runtime"
	"slices"

	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/urlutil"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:     "open [service]",
	Short:   "Open web services in the browser",
	Long:    "Opens all web services (those with a hostname) for the current project in your default browser. Specify a service name to open just one.",
	GroupID: "project",
	Args:    MaximumArgs(1, "accepts at most one service name"),
	RunE:    runOpen,
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
		return fmt.Errorf("No ports allocated. Run 'outport up' first.")
	}

	httpsEnabled := certmanager.IsCAInstalled()

	if len(args) == 1 {
		return openService(cmd, ctx.Cfg, alloc, args[0], httpsEnabled)
	}

	opened := 0
	for _, svcName := range slices.Sorted(maps.Keys(ctx.Cfg.Services)) {
		svc := ctx.Cfg.Services[svcName]
		if svc.Hostname == "" {
			continue
		}
		h := svc.Hostname
		if allocated, ok := alloc.Hostnames[svcName]; ok {
			h = allocated
		}
		url := fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(h, httpsEnabled), h)
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open %s: %v\n", svcName, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", svcName, url)
		opened++
	}

	if opened == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No web services found. Add 'hostname' to services in outport.yml.")
	}

	return nil
}

func openService(cmd *cobra.Command, cfg *config.Config, alloc registry.Allocation, name string, httpsEnabled bool) error {
	svc, ok := cfg.Services[name]
	if !ok {
		return fmt.Errorf("Service %q not found in outport.yml.", name)
	}

	_, ok = alloc.Ports[name]
	if !ok {
		return fmt.Errorf("No port allocated for %q. Run 'outport up' first.", name)
	}

	if svc.Hostname == "" {
		return fmt.Errorf("Service %q has no hostname. Add 'hostname' to open it in the browser.", name)
	}

	h := svc.Hostname
	if allocated, hok := alloc.Hostnames[name]; hok {
		h = allocated
	}
	url := fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(h, httpsEnabled), h)

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

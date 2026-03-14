package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/worktree"
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
		return fmt.Errorf("No ports allocated. Run 'outport up' first.")
	}

	// If a specific service was requested
	if len(args) == 1 {
		return openService(cmd, cfg, alloc, args[0])
	}

	// Open all HTTP services
	opened := 0
	for _, svcName := range sortedMapKeys(cfg.Services) {
		svc := cfg.Services[svcName]
		url := serviceURL(svc.Protocol, alloc.Ports[svcName])
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
		return fmt.Errorf("No port allocated for %q. Run 'outport up' first.", name)
	}

	url := serviceURL(svc.Protocol, port)
	if url == "" {
		return fmt.Errorf("Service %q has no protocol set. Add 'protocol: http' to open it in the browser.", name)
	}

	if err := openBrowser(url); err != nil {
		return fmt.Errorf("Could not open browser: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", name, url)
	return nil
}

// openBrowser opens a URL in the default browser, cross-platform.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}

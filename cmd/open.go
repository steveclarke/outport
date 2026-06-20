package cmd

import (
	"fmt"
	"maps"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/urlutil"
)

var openCmd = &cobra.Command{
	Use:     "open [target]",
	Short:   "Open web services in the browser",
	Long:    "Opens web services for the current project in your default browser. By default, opens all services with a hostname. If the 'open' field is set in outport.yml, only the listed services are opened. Specify a service name, alias name, or service:alias target to open just that one.",
	GroupID: "project",
	Args:    MaximumArgs(1, "accepts at most one target"),
	RunE:    runOpen,
}

var openBrowserFunc = openBrowser

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
		target, err := resolveOpenTarget(ctx.Cfg, alloc, args[0], httpsEnabled)
		if err != nil {
			return err
		}
		if err := openResolvedTarget(cmd, target); err != nil {
			return err
		}
		if jsonFlag {
			return printOpenJSON(cmd, []openTarget{target})
		}
		return nil
	}

	// Determine which services to open
	var serviceNames []string
	if len(ctx.Cfg.Open) > 0 {
		// Config specifies which services to open — use that order
		serviceNames = ctx.Cfg.Open
	} else {
		// No open list — open all services with hostnames (alphabetical)
		for _, name := range slices.Sorted(maps.Keys(ctx.Cfg.Services)) {
			if ctx.Cfg.Services[name].Hostname != "" {
				serviceNames = append(serviceNames, name)
			}
		}
	}

	opened := 0
	var targets []openTarget
	for _, svcName := range serviceNames {
		target, err := resolveServiceTarget(ctx.Cfg, alloc, svcName, httpsEnabled)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open %s: %v\n", svcName, err)
			continue
		}
		if err := openResolvedTarget(cmd, target); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open %s: %v\n", svcName, err)
			continue
		}
		opened++
		targets = append(targets, target)
	}

	if jsonFlag {
		return printOpenJSON(cmd, targets)
	}

	if opened == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No web services found. Add 'hostname' to services in outport.yml.")
	}

	return nil
}

type openTarget struct {
	Kind     string `json:"kind"`
	Service  string `json:"service"`
	Alias    string `json:"alias,omitempty"`
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	Port     int    `json:"port"`
}

func resolveOpenTarget(cfg *config.Config, alloc registry.Allocation, name string, httpsEnabled bool) (openTarget, error) {
	if serviceName, aliasName, ok := strings.Cut(name, ":"); ok {
		if serviceName == "" || aliasName == "" || strings.Contains(aliasName, ":") {
			return openTarget{}, fmt.Errorf("Open target %q must use service:alias.", name)
		}
		return resolveAliasTarget(cfg, alloc, serviceName, aliasName, httpsEnabled)
	}

	if _, ok := cfg.Services[name]; ok {
		return resolveServiceTarget(cfg, alloc, name, httpsEnabled)
	}

	return resolveBareAliasTarget(cfg, alloc, name, httpsEnabled)
}

// servicePort verifies the service exists and has an allocated port.
func servicePort(cfg *config.Config, alloc registry.Allocation, name string) (int, error) {
	if _, ok := cfg.Services[name]; !ok {
		return 0, fmt.Errorf("Service %q not found in outport.yml.", name)
	}
	port, ok := alloc.Ports[name]
	if !ok {
		return 0, fmt.Errorf("No port allocated for %q. Run 'outport up' first.", name)
	}
	return port, nil
}

func resolveServiceTarget(cfg *config.Config, alloc registry.Allocation, name string, httpsEnabled bool) (openTarget, error) {
	port, err := servicePort(cfg, alloc, name)
	if err != nil {
		return openTarget{}, err
	}

	if cfg.Services[name].Hostname == "" {
		return openTarget{}, fmt.Errorf("Service %q has no hostname. Add 'hostname' to open it in the browser.", name)
	}

	h := allocHostname(alloc, cfg, name)
	return openTarget{
		Kind:     "service",
		Service:  name,
		Hostname: h,
		URL:      urlutil.ServiceURL(h, port, httpsEnabled),
		Port:     port,
	}, nil
}

func resolveBareAliasTarget(cfg *config.Config, alloc registry.Allocation, aliasName string, httpsEnabled bool) (openTarget, error) {
	var matches []string
	for _, svcName := range slices.Sorted(maps.Keys(alloc.Aliases)) {
		if _, ok := alloc.Aliases[svcName][aliasName]; ok {
			matches = append(matches, svcName+":"+aliasName)
		}
	}

	if len(matches) == 0 {
		return openTarget{}, fmt.Errorf("Service or alias %q not found in outport.yml.", aliasName)
	}
	if len(matches) > 1 {
		return openTarget{}, fmt.Errorf("alias %q is ambiguous; use one of: %s.", aliasName, strings.Join(matches, ", "))
	}

	serviceName, _, _ := strings.Cut(matches[0], ":")
	return resolveAliasTarget(cfg, alloc, serviceName, aliasName, httpsEnabled)
}

func resolveAliasTarget(cfg *config.Config, alloc registry.Allocation, serviceName, aliasName string, httpsEnabled bool) (openTarget, error) {
	port, err := servicePort(cfg, alloc, serviceName)
	if err != nil {
		return openTarget{}, err
	}

	hostname, ok := alloc.Aliases[serviceName][aliasName]
	if !ok {
		return openTarget{}, fmt.Errorf("Service %q has no alias %q.", serviceName, aliasName)
	}

	return openTarget{
		Kind:     "alias",
		Service:  serviceName,
		Alias:    aliasName,
		Hostname: hostname,
		URL:      urlutil.ServiceURL(hostname, port, httpsEnabled),
		Port:     port,
	}, nil
}

func openResolvedTarget(cmd *cobra.Command, target openTarget) error {
	if err := openBrowserFunc(target.URL); err != nil {
		return fmt.Errorf("Could not open browser: %w.", err)
	}
	if !jsonFlag {
		fmt.Fprintf(cmd.OutOrStdout(), "Opened %s → %s\n", target.label(), target.URL)
	}
	return nil
}

func (t openTarget) label() string {
	if t.Kind == "alias" {
		return t.Service + ":" + t.Alias
	}
	return t.Service
}

func printOpenJSON(cmd *cobra.Command, targets []openTarget) error {
	out := struct {
		Opened []openTarget `json:"opened"`
	}{
		Opened: targets,
	}
	n := len(targets)
	return writeJSON(cmd, out, fmt.Sprintf("%d %s opened", n, pluralize(n, "target", "targets")))
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

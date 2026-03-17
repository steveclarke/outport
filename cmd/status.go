package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var statusCheckFlag bool
var statusDerivedFlag bool

var systemStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all registered projects",
	Args:  NoArgs,
	RunE:  runStatus,
}

func init() {
	systemStatusCmd.Flags().BoolVar(&statusCheckFlag, "check", false, "check if ports are accepting connections")
	systemStatusCmd.Flags().BoolVar(&statusDerivedFlag, "derived", false, "show derived values")
	systemCmd.AddCommand(systemStatusCmd)
}

func currentProjectKey(reg *registry.Registry) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir, err := config.FindDir(cwd)
	if err != nil {
		return ""
	}
	key, _, ok := reg.FindByDir(dir)
	if !ok {
		return ""
	}
	return key
}

func loadProjectConfig(projectDir string) *config.Config {
	cfg, err := config.Load(projectDir)
	if err != nil {
		return nil
	}
	return cfg
}

// isStale checks whether a registry entry's project directory or config
// no longer exists, returning a reason string if stale.
func isStale(projectDir string) (bool, string) {
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return true, "(not found)"
	}
	if loadProjectConfig(projectDir) == nil {
		return true, "(config missing)"
	}
	return false, ""
}

// formatProjectKey returns just the project name for main instances,
// or "project/instance" for non-main instances.
func formatProjectKey(key string) string {
	project, instance := registry.ParseKey(key)
	if instance == "main" {
		return project
	}
	return project + "/" + instance
}

func runStatus(cmd *cobra.Command, args []string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	if len(reg.Projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport up' in a project directory.")
		return nil
	}

	// Precompute port status if --check
	var portStatus map[int]bool
	if statusCheckFlag {
		var allPorts []int
		for _, alloc := range reg.Projects {
			for _, port := range alloc.Ports {
				allPorts = append(allPorts, port)
			}
		}
		portStatus = portcheck.CheckAll(allPorts)
	}

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printStatusJSON(cmd, reg, portStatus, httpsEnabled)
	}
	return printStatusStyled(cmd, reg, portStatus, httpsEnabled)
}

type statusEntryJSON struct {
	Key        string                 `json:"key"`
	ProjectDir string                 `json:"project_dir"`
	Current    bool                   `json:"current"`
	Services   map[string]svcJSON     `json:"services"`
	Derived    map[string]derivedJSON `json:"derived,omitempty"`
}

func printStatusJSON(cmd *cobra.Command, reg *registry.Registry, portStatus map[int]bool, httpsEnabled bool) error {
	currentKey := currentProjectKey(reg)
	var entries []statusEntryJSON

	keys := sortedMapKeys(reg.Projects)
	for _, key := range keys {
		alloc := reg.Projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)
		_, instanceName := registry.ParseKey(key)

		services := make(map[string]svcJSON)
		for svcName, port := range alloc.Ports {
			s := svcJSON{Port: port}
			if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					s.Protocol = svc.Protocol
					s.URL = serviceURL(svc.Protocol, resolvedHostname(svc, alloc.Hostnames, svcName), port, httpsEnabled)
				}
			}
			if portStatus != nil {
				s.Up = boolPtr(portStatus[port])
			}
			services[svcName] = s
		}

		var derived map[string]derivedJSON
		if cfg != nil && statusDerivedFlag {
			derived = buildDerivedMap(cfg.Derived, resolveDerivedFromAlloc(cfg, instanceName, alloc.Ports, alloc.Hostnames, httpsEnabled))
		}

		entries = append(entries, statusEntryJSON{
			Key:        key,
			ProjectDir: alloc.ProjectDir,
			Current:    key == currentKey,
			Services:   services,
			Derived:    derived,
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

var currentMarker = lipgloss.NewStyle().Foreground(ui.Green).Bold(true)

func printStatusStyled(cmd *cobra.Command, reg *registry.Registry, portStatus map[int]bool, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey(reg)

	keys := sortedMapKeys(reg.Projects)

	for i, key := range keys {
		alloc := reg.Projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)
		_, instanceName := registry.ParseKey(key)

		stale, staleReason := isStale(alloc.ProjectDir)

		marker := ""
		if key == currentKey {
			marker = currentMarker.Render(" ●")
		}
		if stale {
			marker += " " + ui.DimStyle.Render(staleReason)
		}
		displayName := formatProjectKey(key)
		header := ui.ProjectStyle.Render(displayName) + " " + ui.DimStyle.Render(alloc.ProjectDir) + marker
		lipgloss.Fprintln(w, header)

		svcNames := sortedMapKeys(alloc.Ports)

		// Use a minimal config for rendering if the real one is missing
		renderCfg := cfg
		if renderCfg == nil {
			renderCfg = &config.Config{Services: make(map[string]config.Service)}
		}
		for _, svcName := range svcNames {
			printServiceLine(w, renderCfg, svcName, alloc.Ports[svcName], alloc.Hostnames, portStatus, httpsEnabled, false)
		}

		if cfg != nil && statusDerivedFlag {
			if resolved := resolveDerivedFromAlloc(cfg, instanceName, alloc.Ports, alloc.Hostnames, httpsEnabled); len(resolved) > 0 {
				printDerivedValues(w, resolved)
			}
		}

		// Show stale hint
		if stale {
			fmt.Fprintf(w, "  %s\n", ui.DimStyle.Render("(stale — run 'outport system gc' to remove)"))
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	return nil
}

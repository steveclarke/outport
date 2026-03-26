package cmd

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/allocation"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/portcheck"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var statusCheckFlag bool
var statusComputedFlag bool

var systemStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all registered projects",
	Args:  NoArgs,
	RunE:  runStatus,
}

func init() {
	systemStatusCmd.Flags().BoolVar(&statusCheckFlag, "check", false, "check if ports are accepting connections")
	systemStatusCmd.Flags().BoolVar(&statusComputedFlag, "computed", false, "show computed values")
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

	projects := reg.All()

	if len(projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport up' in a project directory.")
		return nil
	}

	// Precompute port status if --check
	var portStatus map[int]bool
	if statusCheckFlag {
		var allPorts []int
		for _, alloc := range projects {
			for _, port := range alloc.Ports {
				allPorts = append(allPorts, port)
			}
		}
		portStatus = portcheck.CheckAll(allPorts)
	}

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printStatusJSON(cmd, reg, projects, portStatus, httpsEnabled)
	}
	return printStatusStyled(cmd, reg, projects, portStatus, httpsEnabled)
}

type statusEntryJSON struct {
	Key        string                  `json:"key"`
	ProjectDir string                  `json:"project_dir"`
	Current    bool                    `json:"current"`
	Services   map[string]svcJSON      `json:"services"`
	Computed   map[string]computedJSON `json:"computed,omitempty"`
}

func printStatusJSON(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, portStatus map[int]bool, httpsEnabled bool) error {
	currentKey := currentProjectKey(reg)
	var entries []statusEntryJSON

	keys := slices.Sorted(maps.Keys(projects))
	for _, key := range keys {
		alloc := projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)
		_, instanceName := registry.ParseKey(key)

		var services map[string]svcJSON
		if cfg != nil {
			services = buildServiceMap(cfg, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled)
		} else {
			services = make(map[string]svcJSON)
			for svcName, port := range alloc.Ports {
				services[svcName] = svcJSON{Port: port}
			}
		}
		if portStatus != nil {
			for svcName, s := range services {
				s.Up = boolPtr(portStatus[s.Port])
				services[svcName] = s
			}
		}

		var computed map[string]computedJSON
		if cfg != nil && statusComputedFlag {
			computed = buildComputedMap(cfg.Computed, allocation.ResolveComputed(cfg, instanceName, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, nil))
		}

		entries = append(entries, statusEntryJSON{
			Key:        key,
			ProjectDir: alloc.ProjectDir,
			Current:    key == currentKey,
			Services:   services,
			Computed:   computed,
		})
	}

	return writeJSON(cmd, entries)
}

var currentMarker = lipgloss.NewStyle().Foreground(ui.Green).Bold(true)

func printStatusStyled(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, portStatus map[int]bool, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey(reg)

	keys := slices.Sorted(maps.Keys(projects))

	for i, key := range keys {
		alloc := projects[key]
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

		svcNames := slices.Sorted(maps.Keys(alloc.Ports))

		// Use a minimal config for rendering if the real one is missing
		renderCfg := cfg
		if renderCfg == nil {
			renderCfg = &config.Config{Services: make(map[string]config.Service)}
		}
		for _, svcName := range svcNames {
			printServiceLineCompact(w, renderCfg, svcName, alloc.Ports[svcName], alloc.Hostnames, portStatus, httpsEnabled)
			if svcAliases, ok := alloc.Aliases[svcName]; ok {
				printAliasLines(w, svcAliases, alloc.Ports[svcName], httpsEnabled)
			}
		}

		if cfg != nil && statusComputedFlag {
			if resolved := allocation.ResolveComputed(cfg, instanceName, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, nil); len(resolved) > 0 {
				printComputedValues(w, resolved)
			}
		}

		// Show stale hint
		if stale {
			fmt.Fprintf(w, "  %s\n", ui.DimStyle.Render("(stale — run 'outport system prune' to remove)"))
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	return nil
}

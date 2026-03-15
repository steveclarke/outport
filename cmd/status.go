package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var statusCheckFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all registered projects and their ports",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusCheckFlag, "check", false, "check if ports are accepting connections")
	rootCmd.AddCommand(statusCmd)
}

func currentProjectKey() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return ""
	}
	wt, err := worktree.Detect(dir)
	if err != nil {
		return ""
	}
	return cfg.Name + "/" + wt.Instance
}

func loadProjectConfig(projectDir string) *config.Config {
	cfg, err := config.Load(projectDir)
	if err != nil {
		return nil
	}
	return cfg
}

// formatProjectKey returns just the project name for main instances,
// or "project/instance (worktree)" for worktree instances.
func formatProjectKey(key string) string {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 || parts[1] == "main" {
		return parts[0]
	}
	return parts[0] + "/" + parts[1] + " (worktree)"
}

func runStatus(cmd *cobra.Command, args []string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	if len(reg.Projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport apply' in a project directory.")
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

	if jsonFlag {
		return printStatusJSON(cmd, reg, portStatus)
	}
	return printStatusStyled(cmd, reg, portStatus)
}

type statusEntryJSON struct {
	Key        string             `json:"key"`
	ProjectDir string             `json:"project_dir"`
	Current    bool               `json:"current"`
	Services   map[string]svcJSON `json:"services"`
}

func printStatusJSON(cmd *cobra.Command, reg *registry.Registry, portStatus map[int]bool) error {
	currentKey := currentProjectKey()
	var entries []statusEntryJSON

	keys := sortedMapKeys(reg.Projects)
	for _, key := range keys {
		alloc := reg.Projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)

		services := make(map[string]svcJSON)
		for svcName, port := range alloc.Ports {
			s := svcJSON{Port: port}
			if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					s.Protocol = svc.Protocol
					s.URL = serviceURL(svc.Protocol, port)
				}
			}
			if portStatus != nil {
				s.Up = boolPtr(portStatus[port])
			}
			services[svcName] = s
		}

		entries = append(entries, statusEntryJSON{
			Key:        key,
			ProjectDir: alloc.ProjectDir,
			Current:    key == currentKey,
			Services:   services,
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

func printStatusStyled(cmd *cobra.Command, reg *registry.Registry, portStatus map[int]bool) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey()

	keys := sortedMapKeys(reg.Projects)
	var staleKeys []string

	for i, key := range keys {
		alloc := reg.Projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)

		// Detect stale: directory missing or config missing
		stale := false
		staleReason := ""
		if _, err := os.Stat(alloc.ProjectDir); os.IsNotExist(err) {
			stale = true
			staleReason = "(not found)"
		} else if cfg == nil {
			stale = true
			staleReason = "(config missing)"
		}
		if stale {
			staleKeys = append(staleKeys, key)
		}

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

		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]

			status := ""
			if portStatus != nil {
				if portStatus[port] {
					status = "  " + ui.StatusUp
				} else {
					status = "  " + ui.StatusDown
				}
			}

			url := ""
			if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					if u := serviceURL(svc.Protocol, port); u != "" {
						url = "  " + ui.UrlStyle.Render(u)
					}
				}
			}

			line := fmt.Sprintf("  %s  %s %-5s%s%s",
				ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
				ui.Arrow,
				ui.PortStyle.Render(fmt.Sprintf("%d", port)),
				status,
				url,
			)
			lipgloss.Fprintln(w, line)
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	// Prompt to remove stale entries
	if len(staleKeys) > 0 {
		lipgloss.Fprintln(w)
		removed := false
		for _, key := range staleKeys {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Remove stale project %s from registry?", key)).
				Affirmative("Yes").
				Negative("No").
				Value(&confirm).
				Run()
			if err != nil {
				break
			}

			if confirm {
				parts := strings.SplitN(key, "/", 2)
				if len(parts) == 2 {
					reg.Remove(parts[0], parts[1])
				}
				fmt.Fprintf(w, "  Removed %s.\n", key)
				removed = true
			}
		}
		if removed {
			if err := reg.Save(); err != nil {
				return fmt.Errorf("Could not save registry: %w.", err)
			}
		}
	}

	return nil
}

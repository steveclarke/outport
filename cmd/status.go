package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

// currentProjectKey returns the registry key for the current directory, or "" if not in a project.
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

// loadProjectConfig attempts to load a project's config from its directory.
// Returns nil if the directory or config doesn't exist.
func loadProjectConfig(projectDir string) *config.Config {
	cfg, err := config.Load(projectDir)
	if err != nil {
		return nil
	}
	return cfg
}

func runStatus(cmd *cobra.Command, args []string) error {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	if len(reg.Projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport up' in a project directory.")
		return nil
	}

	if jsonFlag {
		return printStatusJSON(cmd, reg)
	}
	return printStatusStyled(cmd, reg)
}

type statusServiceJSON struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	URL      string `json:"url,omitempty"`
	Up       *bool  `json:"up,omitempty"`
}

type statusEntryJSON struct {
	Key        string                        `json:"key"`
	ProjectDir string                        `json:"project_dir"`
	Current    bool                          `json:"current"`
	Services   map[string]statusServiceJSON  `json:"services"`
}

func printStatusJSON(cmd *cobra.Command, reg *registry.Registry) error {
	currentKey := currentProjectKey()
	var entries []statusEntryJSON

	keys := sortedMapKeys(reg.Projects)
	for _, key := range keys {
		alloc := reg.Projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)

		services := make(map[string]statusServiceJSON)
		for svcName, port := range alloc.Ports {
			s := statusServiceJSON{Port: port}
			if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					s.Protocol = svc.Protocol
					s.URL = serviceURL(svc.Protocol, port)
				}
			}
			if statusCheckFlag {
				s.Up = boolPtr(portcheck.IsUp(port))
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

var (
	currentMarker = lipgloss.NewStyle().Foreground(ui.Green).Bold(true)
)

func printStatusStyled(cmd *cobra.Command, reg *registry.Registry) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey()

	keys := sortedMapKeys(reg.Projects)
	var staleKeys []string

	for i, key := range keys {
		alloc := reg.Projects[key]
		dirExists := true
		if _, err := os.Stat(alloc.ProjectDir); os.IsNotExist(err) {
			dirExists = false
			staleKeys = append(staleKeys, key)
		}

		cfg := loadProjectConfig(alloc.ProjectDir)

		// Header with current project indicator and stale warning
		marker := ""
		if key == currentKey {
			marker = currentMarker.Render(" ●")
		}
		if !dirExists {
			marker += " " + ui.DimStyle.Render("(not found)")
		}
		header := ui.ProjectStyle.Render(key) + " " + ui.DimStyle.Render(alloc.ProjectDir) + marker
		lipgloss.Fprintln(w, header)

		svcNames := sortedMapKeys(alloc.Ports)

		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]

			status := ""
			if statusCheckFlag {
				if portcheck.IsUp(port) {
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
		reader := bufio.NewReader(os.Stdin)
		for _, key := range staleKeys {
			fmt.Fprintf(w, "Remove stale project %s from registry? [y/N]: ", ui.ProjectStyle.Render(key))
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) == "y" {
				parts := strings.SplitN(key, "/", 2)
				if len(parts) == 2 {
					reg.Remove(parts[0], parts[1])
				}
				fmt.Fprintf(w, "  Removed %s.\n", key)
			}
		}
		if err := reg.Save(); err != nil {
			return fmt.Errorf("Could not save registry: %w.", err)
		}
	}

	return nil
}

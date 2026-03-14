package cmd

import (
	"encoding/json"
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all registered projects and their ports",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
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

type statusEntry struct {
	Key        string         `json:"key"`
	ProjectDir string         `json:"project_dir"`
	Ports      map[string]int `json:"ports"`
}

func printStatusJSON(cmd *cobra.Command, reg *registry.Registry) error {
	var entries []statusEntry
	keys := sortedMapKeys(reg.Projects)
	for _, key := range keys {
		alloc := reg.Projects[key]
		entries = append(entries, statusEntry{
			Key:        key,
			ProjectDir: alloc.ProjectDir,
			Ports:      alloc.Ports,
		})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printStatusStyled(cmd *cobra.Command, reg *registry.Registry) error {
	w := cmd.OutOrStdout()

	keys := sortedMapKeys(reg.Projects)

	for i, key := range keys {
		alloc := reg.Projects[key]

		header := ui.ProjectStyle.Render(key) + " " + ui.DimStyle.Render(alloc.ProjectDir)
		lipgloss.Fprintln(w, header)

		svcNames := sortedMapKeys(alloc.Ports)

		for _, svcName := range svcNames {
			line := fmt.Sprintf("  %s  %s %s",
				ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
				ui.Arrow,
				ui.PortStyle.Render(fmt.Sprintf("%d", alloc.Ports[svcName])),
			)
			lipgloss.Fprintln(w, line)
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	return nil
}

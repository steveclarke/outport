package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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
	keys := sortedKeys(reg)
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

	keys := sortedKeys(reg)

	var rows [][]string
	for _, key := range keys {
		alloc := reg.Projects[key]

		svcNames := make([]string, 0, len(alloc.Ports))
		for s := range alloc.Ports {
			svcNames = append(svcNames, s)
		}
		sort.Strings(svcNames)

		var portParts []string
		for _, s := range svcNames {
			portParts = append(portParts, fmt.Sprintf("%s:%d", s, alloc.Ports[s]))
		}

		rows = append(rows, []string{key, alloc.ProjectDir, strings.Join(portParts, ", ")})
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(ui.Purple).
		Bold(true).
		Padding(0, 1)

	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.Purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers("PROJECT", "DIRECTORY", "PORTS").
		Rows(rows...)

	lipgloss.Fprintln(w, t)
	return nil
}

func sortedKeys(reg *registry.Registry) []string {
	keys := make([]string, 0, len(reg.Projects))
	for k := range reg.Projects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

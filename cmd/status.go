package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/outport-app/outport/internal/registry"
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

	keys := make([]string, 0, len(reg.Projects))
	for k := range reg.Projects {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		alloc := reg.Projects[key]
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", key)
		fmt.Fprintf(cmd.OutOrStdout(), "  dir: %s\n", alloc.ProjectDir)

		svcNames := make([]string, 0, len(alloc.Ports))
		for s := range alloc.Ports {
			svcNames = append(svcNames, s)
		}
		sort.Strings(svcNames)
		portStrs := make([]string, 0, len(svcNames))
		for _, s := range svcNames {
			portStrs = append(portStrs, fmt.Sprintf("%s:%d", s, alloc.Ports[s]))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ports: %s\n", strings.Join(portStrs, ", "))
	}

	return nil
}

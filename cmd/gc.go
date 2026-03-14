package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/registry"
	"github.com/spf13/cobra"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Remove stale entries from the registry",
	Long:  "Scans the registry and removes entries whose project directories no longer exist.",
	RunE:  runGC,
}

func init() {
	rootCmd.AddCommand(gcCmd)
}

func runGC(cmd *cobra.Command, args []string) error {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	var removed []string
	for key, alloc := range reg.Projects {
		if _, err := os.Stat(alloc.ProjectDir); os.IsNotExist(err) {
			removed = append(removed, key)
			delete(reg.Projects, key)
		}
	}

	if len(removed) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No stale entries found.")
		return nil
	}

	if err := reg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed %d stale entries:\n", len(removed))
	for _, key := range removed {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", key)
	}

	return nil
}

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var systemPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove stale and duplicate entries from the registry",
	Long:  "Scans the registry and removes entries whose project directories or config files no longer exist, plus duplicate-directory phantoms (multiple entries pointing at the same directory).",
	Args:  NoArgs,
	RunE:  runPrune,
}

func init() {
	systemCmd.AddCommand(systemPruneCmd)
}

func runPrune(cmd *cobra.Command, args []string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	stale := reg.RemoveStale(func(projectDir string) bool {
		s, _ := isStale(projectDir)
		return s
	})
	dupes := reg.PruneDuplicateDirs()

	if len(stale) == 0 && len(dupes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No stale or duplicate entries found.")
		return nil
	}

	if err := reg.Save(); err != nil {
		return err
	}

	printPruned(cmd.OutOrStdout(), "stale", stale)
	printPruned(cmd.OutOrStdout(), "duplicate", dupes)

	return nil
}

func printPruned(w io.Writer, label string, keys []string) {
	if len(keys) == 0 {
		return
	}
	fmt.Fprintf(w, "Removed %d %s %s:\n", len(keys), label, pluralize(len(keys), "entry", "entries"))
	for _, key := range keys {
		fmt.Fprintf(w, "  %s\n", key)
	}
}

package cmd

import (
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear and re-allocate ports for the current project",
	Long:  "Removes existing port allocations and runs a fresh allocation, respecting preferred_port settings. Equivalent to 'outport up --force'.",
	RunE:  runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	forceFlag = true
	defer func() { forceFlag = false }()
	return runUp(cmd, args)
}

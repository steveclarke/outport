package cmd

import (
	"fmt"

	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the daemon",
	Long:  "Loads the LaunchAgent to start the DNS resolver and HTTP proxy daemon.",
	RunE:  runUp,
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the daemon",
	Long:  "Unloads the LaunchAgent to stop the DNS resolver and HTTP proxy daemon.",
	RunE:  runDown,
}

func init() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	if !platform.IsSetup() {
		return fmt.Errorf("outport is not set up. Run 'outport setup' first")
	}

	if err := platform.LoadAgent(); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), ui.SuccessStyle.Render("Daemon started."))
	return nil
}

func runDown(cmd *cobra.Command, args []string) error {
	if err := platform.UnloadAgent(); err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), ui.SuccessStyle.Render("Daemon stopped."))
	return nil
}

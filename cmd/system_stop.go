package cmd

import (
	"fmt"
	"os/exec"

	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var systemStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the outport system",
	Long:  "Unloads the LaunchAgent to stop the DNS resolver and HTTP proxy daemon.",
	Args:  NoArgs,
	RunE:  runSystemStop,
}

var systemRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the outport system",
	Long:  "Re-writes the LaunchAgent configuration and restarts the daemon. Use after upgrading outport.",
	Args:  NoArgs,
	RunE:  runSystemRestart,
}

func init() {
	systemCmd.AddCommand(systemStopCmd)
	systemCmd.AddCommand(systemRestartCmd)
}

func requireSetup() error {
	if !platform.IsSetup() {
		return fmt.Errorf("outport is not set up. Run 'outport system start' first")
	}
	return nil
}

func resolveAndWritePlist() error {
	outportBin, err := exec.LookPath("outport")
	if err != nil {
		return fmt.Errorf("could not find outport binary in PATH: %w", err)
	}
	return platform.WritePlist(outportBin)
}

func runSystemStop(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if err := requireSetup(); err != nil {
		return err
	}

	if !platform.IsAgentLoaded() {
		if jsonFlag {
			return printSystemStatusJSON(w, "already_stopped")
		}
		fmt.Fprintln(w, "Outport system is not running.")
		return nil
	}

	if err := platform.UnloadAgent(); err != nil {
		return err
	}

	if jsonFlag {
		return printSystemStatusJSON(w, "stopped")
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system stopped."))
	return nil
}

func runSystemRestart(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if err := requireSetup(); err != nil {
		return err
	}

	// Re-write plist to pick up new binary path after upgrades
	if err := resolveAndWritePlist(); err != nil {
		return err
	}

	// Stop if running
	if platform.IsAgentLoaded() {
		if err := platform.UnloadAgent(); err != nil {
			return err
		}
	}

	// Start
	if err := platform.LoadAgent(); err != nil {
		return err
	}

	if jsonFlag {
		return printSystemStatusJSON(w, "restarted")
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system restarted."))
	return nil
}

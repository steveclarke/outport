package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/steveclarke/outport/internal/platform"
	"github.com/steveclarke/outport/internal/ui"
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
	// Use the currently running binary, not whatever "outport" resolves to
	// in PATH. exec.LookPath can resolve to shims (e.g., mise) that become
	// stale when the underlying tool is uninstalled, leaving the daemon
	// pointing at a dead path.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine outport binary path: %w", err)
	}
	outportBin, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("could not resolve outport binary path: %w", err)
	}
	if err := platform.WritePlist(outportBin); err != nil {
		return err
	}
	// Re-apply port capabilities on every write — setcap is lost when the
	// binary is replaced (new inode). No-op on macOS (launchd handles ports).
	return platform.EnsurePrivilegedPorts(outportBin)
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

	if err := restartDaemon(); err != nil {
		return err
	}

	if jsonFlag {
		return printSystemStatusJSON(w, "restarted")
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system restarted."))
	return nil
}

package cmd

import (
	"fmt"
	"os/exec"

	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install the DNS resolver and daemon",
	Long:  "Installs the .test DNS resolver and LaunchAgent so that *.test hostnames resolve to your local services.",
	RunE:  runSetup,
}

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Remove the DNS resolver and daemon",
	Long:  "Unloads the daemon, removes the LaunchAgent plist, and removes the .test DNS resolver file.",
	RunE:  runTeardown,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(teardownCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if platform.IsSetup() {
		fmt.Fprintln(w, "Already set up. Use 'outport teardown' to remove and re-install.")
		return nil
	}

	// Check if port 80 is in use
	if isPort80InUse() {
		return fmt.Errorf("port 80 is already in use — stop the other server first")
	}

	// Find the outport binary
	outportBin, err := exec.LookPath("outport")
	if err != nil {
		return fmt.Errorf("could not find outport binary in PATH: %w", err)
	}

	// Install LaunchAgent plist (no sudo needed)
	fmt.Fprintln(w, "Installing LaunchAgent...")
	if err := platform.WritePlist(outportBin); err != nil {
		return err
	}

	// Create resolver file (needs sudo)
	fmt.Fprintln(w, "Creating /etc/resolver/test (sudo may prompt for your password)...")
	if err := platform.WriteResolverFile(); err != nil {
		return err
	}

	// Load the agent
	fmt.Fprintln(w, "Loading daemon...")
	if err := platform.LoadAgent(); err != nil {
		return err
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Setup complete. *.test domains will resolve to your local services."))
	return nil
}

func runTeardown(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// Unload agent (best effort)
	fmt.Fprintln(w, "Unloading daemon...")
	_ = platform.UnloadAgent()

	// Remove plist (best effort)
	fmt.Fprintln(w, "Removing LaunchAgent...")
	_ = platform.RemovePlist()

	// Remove resolver file (uses sudo)
	fmt.Fprintln(w, "Removing /etc/resolver/test (sudo may prompt for your password)...")
	if err := platform.RemoveResolverFile(); err != nil {
		return err
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Teardown complete. DNS resolver and daemon removed."))
	return nil
}

// isPort80InUse checks if anything is listening on TCP port 80.
func isPort80InUse() bool {
	out, err := exec.Command("lsof", "-i", ":80", "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return false // lsof exits non-zero when nothing found
	}
	return len(out) > 0
}

package cmd

import (
	"fmt"

	"charm.land/huh/v2"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "Set up outport on this machine",
	Long:    "Interactive first-run setup. Asks whether to enable .test domains with HTTPS (requires sudo and a one-time keychain prompt). Without .test domains, outport up still works for deterministic ports and .env files.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// setupTheme returns a huh ThemeFunc styled with Outport brand colors.
func setupTheme() huh.ThemeFunc {
	return func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)
		t.Focused.Title = t.Focused.Title.Foreground(ui.Brand).Bold(true)
		t.Focused.FocusedButton = t.Focused.FocusedButton.Background(ui.Brand)
		t.Focused.Description = t.Focused.Description.Foreground(ui.Gray)
		return t
	}
}

func printSetupNextStep(cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run "+ui.ServiceStyle.Render("outport init")+" in any project to get started.")
}

func runSetup(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// JSON mode: non-interactive, delegate entirely to system start
	if jsonFlag {
		return runSystemStart(cmd, args)
	}

	// Interactive prompt
	enableDNS := true
	confirm := huh.NewConfirm().
		Title("Enable .test domains with HTTPS?").
		Description(
			"This adds local DNS, a reverse proxy, and automatic HTTPS\n" +
				"for .test hostnames. Requires sudo and a one-time keychain prompt.\n\n" +
				"Without it, outport up still works — you get deterministic\n" +
				"ports and .env files with zero system changes.",
		).
		Affirmative("Yes").
		Negative("No").
		Value(&enableDNS)

	err := huh.NewForm(huh.NewGroup(confirm)).
		WithTheme(setupTheme()).
		WithShowHelp(false).
		Run()
	if err != nil {
		return err
	}

	if !enableDNS {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.SuccessStyle.Render("Setup complete."))
		fmt.Fprintln(w, ui.DimStyle.Render("Tip: You can enable .test domains later with outport system start."))
		printSetupNextStep(cmd)
		return nil
	}

	// Delegate to the existing system start logic — it handles all cases
	// (first-time setup, already installed but stopped, idempotent re-runs)
	fmt.Fprintln(w)
	if err := runSystemStart(cmd, args); err != nil {
		return err
	}

	printSetupNextStep(cmd)
	return nil
}

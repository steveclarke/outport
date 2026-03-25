package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/huh/v2"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/platform"
	"github.com/steveclarke/outport/internal/settings"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var resetFlag bool

var setupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "Set up outport on this machine",
	Long:    "Interactive first-run setup. Asks whether to enable .test domains with HTTPS (requires sudo and a one-time keychain prompt). Without .test domains, outport up still works for deterministic ports and .env files.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&resetFlag, "reset", false, "tear down and re-run setup from scratch")
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

// ensureConfigFile creates ~/.config/outport/config with commented-out defaults
// if it doesn't already exist. Never overwrites an existing file.
func ensureConfigFile() error {
	path, err := settings.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return os.WriteFile(path, []byte(settings.DefaultConfigContent()), 0644)
}

func printSetupNextStep(cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run "+ui.ServiceStyle.Render("outport init")+" in any project to get started.")
}

func runSetup(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// --reset: tear down first, then proceed with fresh setup
	if resetFlag {
		fmt.Fprintln(w, "Resetting system...")
		if err := runSystemUninstall(cmd, args); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
		fmt.Fprintln(w)
	}

	if err := ensureConfigFile(); err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}

	// Already fully set up — skip the prompt entirely
	if platform.IsSetup() && certmanager.IsCAInstalled() && platform.IsAgentLoaded() {
		if jsonFlag {
			return printSystemStatusJSON(w, "already_running")
		}
		fmt.Fprintln(w, ui.SuccessStyle.Render("✓ Already set up. Nothing to do."))
		fmt.Fprintln(w, ui.DimStyle.Render("  Run outport setup --reset to tear down and re-run setup."))
		return nil
	}

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
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrSilent
		}
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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .outport.yml for this project",
	Long:  "Interactively creates an .outport.yml configuration file in the current directory.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

type servicePreset struct {
	Name          string
	PreferredPort int
	EnvVar        string
	Protocol      string
}

var presets = []servicePreset{
	{"web", 3000, "PORT", "http"},
	{"postgres", 5432, "DATABASE_PORT", ""},
	{"redis", 6379, "REDIS_PORT", ""},
	{"mailpit_web", 8025, "MAILPIT_WEB_PORT", "http"},
	{"mailpit_smtp", 1025, "MAILPIT_SMTP_PORT", ""},
	{"vite", 5173, "VITE_PORT", "http"},
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	configPath := filepath.Join(dir, config.FileName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists.", config.FileName)
	}

	dirName := filepath.Base(dir)
	name := dirName

	huh.NewInput().
		Title("Project name").
		Value(&name).
		Run()

	// Build multi-select options from presets
	var options []huh.Option[string]
	for _, p := range presets {
		label := fmt.Sprintf("%s (port %d)", p.Name, p.PreferredPort)
		options = append(options, huh.NewOption(label, p.Name))
	}

	var selected []string
	huh.NewMultiSelect[string]().
		Title("Select services").
		Options(options...).
		Value(&selected).
		Run()

	// Map selected names back to presets
	var selectedServices []servicePreset
	for _, name := range selected {
		for _, p := range presets {
			if p.Name == name {
				selectedServices = append(selectedServices, p)
				break
			}
		}
	}

	if len(selectedServices) == 0 {
		selectedServices = []servicePreset{presets[0]}
		fmt.Fprintln(cmd.OutOrStdout(), "No services selected, defaulting to web.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString("services:\n")
	for _, svc := range selectedServices {
		sb.WriteString(fmt.Sprintf("  %s:\n", svc.Name))
		sb.WriteString(fmt.Sprintf("    preferred_port: %d\n", svc.PreferredPort))
		sb.WriteString(fmt.Sprintf("    env_var: %s\n", svc.EnvVar))
		if svc.Protocol != "" {
			sb.WriteString(fmt.Sprintf("    protocol: %s\n", svc.Protocol))
		}
	}

	if err := os.WriteFile(configPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("Writing config: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nCreated %s\n", config.FileName)
	fmt.Fprintln(cmd.OutOrStdout(), "Run 'outport up' to allocate ports.")

	return nil
}

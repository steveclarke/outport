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
	Name     string
	EnvVar   string
	Protocol string
}

var presets = []servicePreset{
	{"web", "PORT", "http"},
	{"postgres", "DATABASE_PORT", ""},
	{"redis", "REDIS_PORT", ""},
	{"mailpit_web", "MAILPIT_WEB_PORT", "http"},
	{"mailpit_smtp", "MAILPIT_SMTP_PORT", ""},
	{"vite", "VITE_PORT", "http"},
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
		options = append(options, huh.NewOption(p.Name, p.Name))
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
		sb.WriteString(fmt.Sprintf("    env_var: %s\n", svc.EnvVar))
		if svc.Protocol != "" {
			sb.WriteString(fmt.Sprintf("    protocol: %s\n", svc.Protocol))
		}
	}

	if err := os.WriteFile(configPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("Writing config: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nCreated %s\n", config.FileName)
	fmt.Fprintln(cmd.OutOrStdout(), "Run 'outport register' to allocate ports.")

	return nil
}

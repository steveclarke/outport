package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/outport-app/outport/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .outport.yml for this project",
	Long:  "Creates a commented .outport.yml template in the current directory.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const configTemplate = `# Outport configuration
# Docs: https://outport.app
#
# Declare your services, then run 'outport register' to allocate ports.
# Outport assigns deterministic ports and writes them to .env as environment variables.
# Your app reads the env vars — Outport doesn't touch your app's config files.

name: %s

services:
# Uncomment and edit the services your project uses:
#
#  web:
#    env_var: PORT
#    protocol: http          # enables 'outport open' and shows URLs in output
#
#  postgres:
#    env_var: DB_PORT
#
#  redis:
#    env_var: REDIS_PORT
#
#  mailpit_web:
#    env_var: MAILPIT_WEB_PORT
#    protocol: http
#
#  mailpit_smtp:
#    env_var: MAILPIT_SMTP_PORT
#
# Write to a different .env file (default is .env in project root):
#
#  rails:
#    env_var: RAILS_PORT
#    protocol: http
#    env_file: backend/.env
#
# Write to multiple .env files:
#
#  postgres:
#    env_var: DB_PORT
#    env_file:
#      - backend/.env
#      - .env
`

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	configPath := filepath.Join(dir, config.FileName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists.", config.FileName)
	}

	name := filepath.Base(dir)
	content := fmt.Sprintf(configTemplate, name)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("Writing config: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", config.FileName)
	fmt.Fprintln(cmd.OutOrStdout(), "Edit it for your project, then run 'outport register' to allocate ports.")

	return nil
}

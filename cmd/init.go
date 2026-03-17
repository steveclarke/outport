package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/outport-app/outport/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Create .outport.yml for this project",
	Long:    "Creates a commented .outport.yml template in the current directory.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const configTemplate = `# Outport configuration
# Docs: https://outport.app
#
# Declare your services, then run 'outport up' to allocate ports.
# Outport assigns deterministic ports and writes them to .env.
# Run 'outport system start' to enable .test domains (e.g., %s.test).

name: %s

services:
  web:
    env_var: PORT
    protocol: http
    hostname: %s.test
#
#  postgres:
#    env_var: DB_PORT
#
# Multiple HTTP services get subdomain hostnames:
#
#  frontend:
#    env_var: FRONTEND_PORT
#    protocol: http
#    hostname: app.%s.test
#
# Write to a different .env file (default is .env in project root):
#
#  rails:
#    env_var: RAILS_PORT
#    env_file: backend/.env

# Derived values — computed env vars that reference allocated ports:
#
# derived:
#  CORS_ORIGINS:
#    value: "${web.url}"
#    env_file: .env
#
#  # Use :direct for server-to-server URLs (bypasses .test proxy):
#  # API_URL: "${web.url:direct}/api/v1"
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
	content := fmt.Sprintf(configTemplate, name, name, name, name)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("Writing config: %w.", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", config.FileName)
	fmt.Fprintln(cmd.OutOrStdout(), "Edit it for your project, then run 'outport up' to allocate ports.")

	return nil
}

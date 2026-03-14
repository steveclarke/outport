package cmd

import (
	"github.com/spf13/cobra"
)

var (
	version  = "dev"
	jsonFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "outport",
	Short: "Dev port manager for multi-project, multi-worktree development",
	Long:  "Outport allocates deterministic, non-conflicting ports for your projects and writes them to .env files.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
}

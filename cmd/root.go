package cmd

import (
	"github.com/spf13/cobra"
)

var (
	version  = "dev"
	jsonFlag bool
)

var rootCmd = &cobra.Command{
	Use:           "outport",
	Short:         "Dev port manager for multi-project development",
	Long:          "Outport allocates deterministic, non-conflicting ports for your projects and writes them to .env files.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	cmd, err := rootCmd.ExecuteC()
	if err != nil && IsFlagError(err) {
		cmd.Println()
		cmd.Println(cmd.UsageString())
	}
	return err
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output in JSON format")

	rootCmd.AddGroup(
		&cobra.Group{ID: "project", Title: "Project Commands:"},
		&cobra.Group{ID: "system", Title: "System Commands:"},
	)

	// Wrap Cobra's flag errors so they trigger usage display
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &FlagError{err: err}
	})
}

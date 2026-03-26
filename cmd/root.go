package cmd

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveclarke/outport/internal/ui"
)

var (
	version  = "dev"
	commit   = ""
	date     = ""
	jsonFlag bool
	yesFlag  bool
)

var rootCmd = &cobra.Command{
	Use:           "outport",
	Short:         "Dev port manager for multi-project development",
	Long:          "Outport allocates deterministic, non-conflicting ports for your projects and writes them to .env files.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		maybeRestartDaemon(cmd)
	},
}

func Execute() error {
	ui.Init()
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		if IsFlagError(err) {
			cmd.Println()
			cmd.Println(cmd.UsageString())
		}
		// When --json is active, emit an error envelope to stdout so
		// machine consumers always get structured output.
		if jsonFlag && !errors.Is(err, ErrSilent) {
			hint := jsonErrorHint(err)
			writeJSONError(cmd.OutOrStdout(), err.Error(), hint)
			return ErrSilent // suppress stderr duplicate from main.go
		}
	}
	return err
}

// jsonErrorHint returns a contextual hint for common errors.
func jsonErrorHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "No outport.yml"):
		return "Run: outport init"
	case strings.Contains(msg, "not registered"):
		return "Run: outport up"
	case strings.Contains(msg, "No ports allocated"):
		return "Run: outport up"
	default:
		return ""
	}
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "auto-approve external env file writes")

	rootCmd.AddGroup(
		&cobra.Group{ID: "project", Title: "Project Commands:"},
		&cobra.Group{ID: "system", Title: "System Commands:"},
	)

	// Wrap Cobra's flag errors so they trigger usage display
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &FlagError{err: err}
	})
}

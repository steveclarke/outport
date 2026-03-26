package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the outport version",
	Args:  NoArgs,
	RunE:  runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

type versionJSON struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
}

func runVersion(cmd *cobra.Command, args []string) error {
	if jsonFlag {
		return printVersionJSON(cmd)
	}
	return printVersionStyled(cmd)
}

func printVersionJSON(cmd *cobra.Command) error {
	return writeJSON(cmd, versionJSON{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, version)
}

func printVersionStyled(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	out := fmt.Sprintf("outport version %s", version)

	if commit != "" || date != "" {
		var details []string
		if commit != "" {
			details = append(details, fmt.Sprintf("commit: %s", commit))
		}
		if date != "" {
			details = append(details, fmt.Sprintf("built: %s", date))
		}
		out += " ("
		for i, d := range details {
			if i > 0 {
				out += ", "
			}
			out += d
		}
		out += ")"
	}

	fmt.Fprintln(w, out)
	return nil
}

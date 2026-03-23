package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:     "system",
	Short:   "Manage the outport system (daemon, DNS, certificates)",
	Long:    "Commands for managing the machine-wide outport installation: daemon lifecycle, DNS resolver, CA certificates, and the global project registry.",
	GroupID: "system",
}

func init() {
	rootCmd.AddCommand(systemCmd)
}

type systemStatusResponse struct {
	Status string `json:"status"`
}

func printSystemStatusJSON(w io.Writer, status string) error {
	return writeJSONTo(w, systemStatusResponse{Status: status})
}

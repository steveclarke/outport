package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/platform"
	"github.com/spf13/cobra"
)

// maybeRestartDaemon checks whether the running daemon's version matches the
// CLI version and silently restarts it if they differ. This ensures users get
// dashboard and proxy updates after a Homebrew upgrade without having to
// remember to run "outport system restart".
//
// Best-effort: any failure is silently ignored so it never blocks a command.
func maybeRestartDaemon(cmd *cobra.Command) {
	// Skip for commands that manage the daemon directly
	if cmd == daemonCmd || cmd == setupCmd || isSystemCommand(cmd) {
		return
	}

	// Skip if the system isn't set up or the daemon isn't running
	if !platform.IsSetup() || !platform.IsAgentLoaded() {
		return
	}

	daemonVersion, err := fetchDaemonVersion()
	if err != nil {
		return
	}

	if daemonVersion == version {
		return
	}

	// Version mismatch — restart the daemon
	if err := resolveAndWritePlist(); err != nil {
		return
	}
	if platform.IsAgentLoaded() {
		if err := platform.UnloadAgent(); err != nil {
			return
		}
	}
	if err := platform.LoadAgent(); err != nil {
		return
	}

	fmt.Fprintf(os.Stderr, "Daemon updated to %s.\n", version)
}

func isSystemCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == systemCmd {
			return true
		}
	}
	return false
}

func fetchDaemonVersion() (string, error) {
	scheme := "http"
	if certmanager.IsCAInstalled() {
		scheme = "https"
	}

	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(scheme + "://outport.test/api/status")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var status struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", err
	}
	return status.Version, nil
}

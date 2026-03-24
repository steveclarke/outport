package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/platform"
	"github.com/spf13/cobra"
)

// maybeRestartDaemon checks whether the running daemon's version matches the
// CLI version and silently restarts it if they differ. This ensures users get
// dashboard and proxy updates after a Homebrew upgrade without having to
// remember to run "outport system restart".
//
// Best-effort: any failure is silently ignored so it never blocks a command.
func maybeRestartDaemon(cmd *cobra.Command) {
	if cmd == daemonCmd || cmd == setupCmd || isSystemCommand(cmd) {
		return
	}

	if !platform.IsSetup() || !platform.IsAgentLoaded() {
		return
	}

	daemonVersion, err := fetchDaemonVersion()
	if err != nil || daemonVersion == version {
		return
	}

	if err := restartDaemon(); err != nil {
		return
	}

	fmt.Fprintf(os.Stderr, "Daemon updated to %s.\n", version)
}

// restartDaemon re-writes the plist and restarts the LaunchAgent.
// Used by both the explicit "system restart" command and the automatic
// version-mismatch restart.
func restartDaemon() error {
	if err := resolveAndWritePlist(); err != nil {
		return err
	}
	if platform.IsAgentLoaded() {
		if err := platform.UnloadAgent(); err != nil {
			return err
		}
	}
	return platform.LoadAgent()
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

	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get(scheme + "://outport.test/api/version")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var data struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	return data.Version, nil
}

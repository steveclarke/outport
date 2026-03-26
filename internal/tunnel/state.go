package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/steveclarke/outport/internal/paths"
)

// StateFilename is the name of the tunnel state file in the data directory.
const StateFilename = "tunnels.json"

// TunnelState represents the persisted state of active tunnels.
type TunnelState struct {
	PID         int                          `json:"pid"`
	Tunnels     map[string]map[string]string `json:"tunnels"`                // key -> service -> URL
	HostnameMap map[string]string            `json:"hostname_map,omitempty"` // tunnel hostname -> .test hostname
}

// DefaultStatePath returns ~/.local/share/outport/tunnels.json.
func DefaultStatePath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StateFilename), nil
}

// WriteState writes tunnel state to the given path with the current PID.
// The hostnameMap parameter maps tunnel hostnames (e.g., "abc123.trycloudflare.com")
// to their corresponding .test hostnames (e.g., "myapp.test"), enabling the daemon
// to build HostOverride proxy routes for active tunnels.
func WriteState(path string, key string, tunnels map[string]string, hostnameMap map[string]string) error {
	state := TunnelState{
		PID:         os.Getpid(),
		Tunnels:     map[string]map[string]string{key: tunnels},
		HostnameMap: hostnameMap,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tunnel state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing tunnel state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming tunnel state: %w", err)
	}
	return nil
}

// ReadState reads tunnel state from the given path.
// Returns nil, nil if the file doesn't exist or the PID is stale.
func ReadState(path string) (*TunnelState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tunnel state: %w", err)
	}
	var state TunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing tunnel state: %w", err)
	}
	if !isProcessAlive(state.PID) {
		return nil, nil
	}
	return &state, nil
}

// RemoveState deletes the tunnel state file (best-effort).
func RemoveState(path string) {
	_ = os.Remove(path)
}

// isProcessAlive checks whether a process with the given PID exists.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

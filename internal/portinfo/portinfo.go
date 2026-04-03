// Package portinfo provides system-level port scanning and process inspection.
// It uses gopsutil to discover listening TCP ports, identify the processes behind
// them, and detect orphaned or zombie dev processes — no shelling out to lsof/ps.
//
// The Lister interface abstracts process discovery so tests can inject fake data.
package portinfo

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"time"
)

// ProcessInfo holds complete information about a process listening on a port.
type ProcessInfo struct {
	PID       int           `json:"pid"`
	PPID      int           `json:"ppid"`
	Name      string        `json:"name"`
	Command   string        `json:"command"`
	Port      int           `json:"port"`
	RSS       int64         `json:"rss_bytes"`
	Elapsed   time.Duration `json:"-"`
	State     string        `json:"-"`
	CWD       string        `json:"cwd,omitempty"`
	Project   string        `json:"project,omitempty"`
	Framework string        `json:"framework,omitempty"`
	IsOrphan  bool          `json:"is_orphan"`
	IsZombie  bool          `json:"is_zombie"`
}

// UptimeSeconds returns the process uptime in seconds for JSON output.
func (p ProcessInfo) UptimeSeconds() int64 {
	return int64(p.Elapsed.Seconds())
}

// Lister abstracts the system calls for discovering listening processes.
// The real implementation (SystemLister) uses gopsutil; tests inject a fake.
type Lister interface {
	// ListProcesses returns info for all processes listening on TCP ports.
	ListProcesses() ([]ProcessInfo, error)
}

// Scan discovers all listening TCP ports and returns enriched process info.
func Scan(lister Lister) ([]ProcessInfo, error) {
	procs, err := lister.ListProcesses()
	if err != nil {
		return nil, err
	}

	for i := range procs {
		enrichProcess(&procs[i])
	}

	sort.Slice(procs, func(i, j int) bool {
		return procs[i].Port < procs[j].Port
	})

	return procs, nil
}

// ScanPorts returns info only for processes listening on the specified ports.
func ScanPorts(ports []int, lister Lister) ([]ProcessInfo, error) {
	filter := make(map[int]bool, len(ports))
	for _, p := range ports {
		filter[p] = true
	}

	procs, err := lister.ListProcesses()
	if err != nil {
		return nil, err
	}

	var filtered []ProcessInfo
	for _, p := range procs {
		if filter[p.Port] {
			enrichProcess(&p)
			filtered = append(filtered, p)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Port < filtered[j].Port
	})

	return filtered, nil
}

// enrichProcess fills in framework detection and orphan/zombie classification.
func enrichProcess(p *ProcessInfo) {
	if p.CWD != "" {
		p.Project, p.Framework = detectFramework(p.CWD)
	}
	p.IsOrphan = isOrphanProcess(p.PPID, p.Name)
	p.IsZombie = isZombieProcess(p.State)
}

// Kill sends SIGTERM to the given PID. Returns an error for protected PIDs.
func Kill(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to kill PID %d", pid)
	}
	if pid == os.Getpid() {
		return fmt.Errorf("refusing to kill outport's own process")
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

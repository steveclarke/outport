// Package portinfo provides system-level port scanning and process inspection.
// It shells out to lsof and ps to discover listening TCP ports, identify the
// processes behind them, and detect orphaned or zombie dev processes.
//
// The Scanner interface abstracts the system calls so tests can inject canned
// output without hitting the real system.
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
	PID       int       `json:"pid"`
	PPID      int       `json:"ppid"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	Port      int       `json:"port"`
	RSS       int64     `json:"rss_bytes"`
	StartTime time.Time `json:"-"`
	State     string    `json:"-"`
	CWD       string    `json:"cwd,omitempty"`
	Project   string    `json:"project,omitempty"`
	Framework string    `json:"framework,omitempty"`
	IsOrphan  bool      `json:"is_orphan"`
	IsZombie  bool      `json:"is_zombie"`
}

// UptimeSeconds returns the process uptime as an integer for JSON output.
func (p ProcessInfo) UptimeSeconds() int64 {
	if p.StartTime.IsZero() {
		return 0
	}
	return int64(time.Since(p.StartTime).Seconds())
}

// Scanner abstracts the system commands used for port discovery.
type Scanner interface {
	ListeningPorts() (string, error)
	ProcessInfo(pids []int) (string, error)
	WorkingDirs(pids []int) (string, error)
}

// Scan discovers all listening TCP ports and returns enriched process info.
func Scan(scanner Scanner) ([]ProcessInfo, error) {
	return scan(scanner, nil)
}

// ScanPorts scans only the specified ports.
func ScanPorts(ports []int, scanner Scanner) ([]ProcessInfo, error) {
	filter := make(map[int]bool, len(ports))
	for _, p := range ports {
		filter[p] = true
	}
	return scan(scanner, filter)
}

func scan(scanner Scanner, portFilter map[int]bool) ([]ProcessInfo, error) {
	lsofOutput, err := scanner.ListeningPorts()
	if err != nil {
		return nil, fmt.Errorf("listing ports: %w", err)
	}

	entries, err := parseLsofListening(lsofOutput)
	if err != nil {
		return nil, fmt.Errorf("parsing lsof: %w", err)
	}

	// Filter to requested ports if specified
	if portFilter != nil {
		var filtered []lsofEntry
		for _, e := range entries {
			if portFilter[e.Port] {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Collect unique PIDs
	pidSet := make(map[int]bool)
	for _, e := range entries {
		pidSet[e.PID] = true
	}
	pids := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}

	// Batch: get process details
	psOutput, err := scanner.ProcessInfo(pids)
	if err != nil {
		return nil, fmt.Errorf("getting process info: %w", err)
	}
	psEntries := parsePsOutput(psOutput)

	// Batch: get working directories (non-fatal if it fails)
	cwdOutput, err := scanner.WorkingDirs(pids)
	if err != nil {
		cwdOutput = ""
	}
	cwds := parseLsofCwd(cwdOutput)

	// Build results
	results := make([]ProcessInfo, 0, len(entries))
	for _, entry := range entries {
		info := ProcessInfo{
			PID:  entry.PID,
			Name: entry.ProcessName,
			Port: entry.Port,
		}

		if ps, ok := psEntries[entry.PID]; ok {
			info.PPID = ps.PPID
			info.State = ps.State
			info.RSS = ps.RSS * 1024 // ps reports KB, we store bytes
			info.StartTime = ps.StartTime
			info.Command = ps.Command
			info.IsOrphan = isOrphanProcess(ps.PPID, entry.ProcessName)
			info.IsZombie = isZombieProcess(ps.State)
		}

		if cwd, ok := cwds[entry.PID]; ok {
			info.CWD = cwd
			info.Project, info.Framework = detectFramework(cwd)
		}

		results = append(results, info)
	}

	// Sort by port for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	return results, nil
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

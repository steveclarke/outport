// Package portcheck provides TCP port liveness probing for localhost services.
// It is used in two main contexts: the dashboard's health checker (which periodically
// polls all allocated ports to display up/down status) and the allocator's collision
// avoidance (which checks whether a port is already bound before assigning it).
//
// All probes target localhost only. The package offers three probe strategies:
// IsUp (dial with standard timeout for health checks), IsBound (dial with shorter
// timeout for the allocation hot path), and IsListening (bind attempt for detecting
// ports in LISTEN state before the daemon starts).
package portcheck

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
)

const (
	// timeout is the TCP dial timeout used by IsUp for health checks. 200ms is
	// generous enough to avoid false negatives on a loaded machine while keeping
	// the total CheckAll wall time under 250ms (all dials run concurrently).
	timeout = 200 * time.Millisecond

	// quickTimeout is the shorter TCP dial timeout used by IsBound during port
	// allocation. The allocator needs to check many candidate ports quickly, so
	// a 100ms timeout keeps the allocation path fast.
	quickTimeout = 100 * time.Millisecond
)

// IsUp checks if a port is accepting TCP connections on localhost by attempting
// a TCP dial with a 200ms timeout. Returns true if the connection succeeds,
// meaning a process is actively listening and accepting connections on that port.
// Used by the dashboard's health checker for periodic liveness probes.
func IsUp(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsBound checks if a port is in use on localhost by attempting a TCP dial with
// a shorter 100ms timeout. This is used by the allocator during port assignment
// to detect ports that are already occupied by other processes, avoiding
// collisions with services not managed by Outport. The shorter timeout keeps
// the allocation path fast when checking multiple candidate ports.
func IsBound(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), quickTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsListening checks if a port is in use by attempting to bind to it.
// Unlike IsBound (which dials), this detects ports in LISTEN state even
// if they aren't yet accepting connections. Used by system start to check
// whether ports 80/443 are available before starting the daemon.
// Permission errors (e.g., binding to port 80 without root) are not
// treated as "in use" — the daemon runs via launchd with socket
// activation, so it doesn't need the calling process to have permission.
func IsListening(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		// Permission denied (EACCES) means the port is available but
		// we can't bind to it as a non-root user. That's fine — the
		// daemon gets the socket from launchd.
		var sysErr *net.OpError
		if errors.As(err, &sysErr) {
			if errors.Is(sysErr.Err, syscall.EACCES) {
				return false
			}
		}
		return true // port is genuinely in use
	}
	ln.Close()
	return false
}

// CheckAll probes all given ports concurrently and returns a map of port to
// up/down status. Every port is dialed in its own goroutine, so the total wall
// time is bounded by the single-dial timeout (~200ms) regardless of how many
// ports are checked. Used by the dashboard's health checker to poll all
// allocated ports in a single batch.
func CheckAll(ports []int) map[int]bool {
	results := make(map[int]bool, len(ports))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			up := IsUp(port)
			mu.Lock()
			results[port] = up
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return results
}

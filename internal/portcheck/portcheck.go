package portcheck

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	timeout      = 200 * time.Millisecond
	quickTimeout = 100 * time.Millisecond
)

// IsUp checks if a port is accepting TCP connections on localhost.
func IsUp(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsBound checks if a port is in use on localhost, using a shorter timeout
// suitable for the allocation hot path.
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
func IsListening(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true // port is in use
	}
	ln.Close()
	return false
}

// CheckAll probes all ports concurrently and returns a map of port → up/down.
// All dials happen in parallel, so worst case is ~200ms regardless of port count.
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

package portcheck

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const timeout = 200 * time.Millisecond

// IsUp checks if a port is accepting TCP connections on localhost.
func IsUp(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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

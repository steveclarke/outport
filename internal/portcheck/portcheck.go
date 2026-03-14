package portcheck

import (
	"fmt"
	"net"
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

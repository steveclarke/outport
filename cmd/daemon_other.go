//go:build !darwin

package cmd

import (
	"fmt"
	"net"
)

func activateLaunchdSocket() (net.Listener, error) {
	return nil, fmt.Errorf("launchd not available on this platform")
}

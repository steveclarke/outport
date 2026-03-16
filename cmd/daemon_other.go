//go:build !darwin

package cmd

import (
	"fmt"
	"net"
)

func activateLaunchdHTTPSocket() (net.Listener, error) {
	return nil, fmt.Errorf("launchd not available on this platform")
}

func activateLaunchdHTTPSSocket() (net.Listener, error) {
	return nil, fmt.Errorf("launchd not available on this platform")
}

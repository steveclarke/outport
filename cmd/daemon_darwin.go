//go:build darwin

package cmd

import (
	"net"

	launchd "github.com/bored-engineer/go-launchd"
)

func activateLaunchdHTTPSocket() (net.Listener, error) {
	return launchd.Activate("HTTPSocket")
}

func activateLaunchdHTTPSSocket() (net.Listener, error) {
	return launchd.Activate("HTTPSSocket")
}

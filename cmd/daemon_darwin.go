//go:build darwin

package cmd

import (
	"net"

	launchd "github.com/bored-engineer/go-launchd"
)

func activateLaunchdSocket() (net.Listener, error) {
	return launchd.Activate("Socket")
}

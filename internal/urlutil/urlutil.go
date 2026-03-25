// Package urlutil provides helpers for constructing service URLs from hostnames,
// ports, and the current HTTPS configuration. It centralizes the URL-building
// logic so that the dashboard, CLI output, and template expansion all produce
// consistent URLs.
//
// The key rule is that .test domains go through the daemon's proxy (which handles
// TLS termination), so they use https:// when HTTPS is enabled and never include
// a port number. Non-.test hostnames are accessed directly and always use http://
// with an explicit port.
package urlutil

import (
	"fmt"
	"strings"
)

// EffectiveScheme returns the URL scheme ("http" or "https") appropriate for
// the given hostname. HTTPS is only used when both conditions are met: the daemon
// has HTTPS enabled (meaning the local CA and certificates are set up) and the
// hostname is a .test domain (which routes through the daemon's TLS-terminating
// proxy). Non-.test hostnames always use plain HTTP because they bypass the proxy.
func EffectiveScheme(hostname string, httpsEnabled bool) string {
	if httpsEnabled && strings.HasSuffix(hostname, ".test") {
		return "https"
	}
	return "http"
}

// ServiceURL returns the browsable URL for a service, or an empty string if the
// service has no hostname (i.e., it is an infrastructure service like a database
// that has no web interface). For .test hostnames, the URL omits the port because
// the daemon's proxy listens on standard ports 80/443. For non-.test hostnames,
// the port is included explicitly (e.g., "http://localhost:3000").
func ServiceURL(hostname string, port int, httpsEnabled bool) string {
	if hostname == "" {
		return ""
	}
	if strings.HasSuffix(hostname, ".test") {
		return fmt.Sprintf("%s://%s", EffectiveScheme(hostname, httpsEnabled), hostname)
	}
	return fmt.Sprintf("http://%s:%d", hostname, port)
}

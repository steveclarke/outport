package urlutil

import (
	"fmt"
	"strings"
)

// EffectiveScheme returns the scheme to use for a given hostname.
// When httpsEnabled is true and the hostname is a .test domain,
// the scheme is "https". Otherwise it's "http".
func EffectiveScheme(hostname string, httpsEnabled bool) string {
	if httpsEnabled && strings.HasSuffix(hostname, ".test") {
		return "https"
	}
	return "http"
}

// ServiceURL returns the browsable URL for a service, or "" if the service
// has no hostname (i.e., it's an infrastructure service).
func ServiceURL(hostname string, port int, httpsEnabled bool) string {
	if hostname == "" {
		return ""
	}
	if strings.HasSuffix(hostname, ".test") {
		return fmt.Sprintf("%s://%s", EffectiveScheme(hostname, httpsEnabled), hostname)
	}
	return fmt.Sprintf("http://%s:%d", hostname, port)
}

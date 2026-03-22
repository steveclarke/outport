package urlutil

import (
	"fmt"
	"strings"
)

// EffectiveScheme returns the scheme to use for a given protocol and hostname.
// When httpsEnabled is true and the hostname is a .test domain with an HTTP protocol,
// the scheme is upgraded to "https".
func EffectiveScheme(protocol, hostname string, httpsEnabled bool) string {
	if httpsEnabled && strings.HasSuffix(hostname, ".test") && (protocol == "http" || protocol == "https") {
		return "https"
	}
	return protocol
}

// ServiceURL returns the browsable URL for a service, or "" if the service
// has no web protocol (i.e., not http or https).
func ServiceURL(protocol, hostname string, port int, httpsEnabled bool) string {
	if protocol == "http" || protocol == "https" {
		host := hostname
		if host == "" {
			host = "localhost"
		}
		if strings.HasSuffix(host, ".test") {
			return fmt.Sprintf("%s://%s", EffectiveScheme(protocol, host, httpsEnabled), host)
		}
		return fmt.Sprintf("%s://%s:%d", protocol, host, port)
	}
	return ""
}

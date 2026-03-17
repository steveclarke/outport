package cloudflare

import "regexp"

// tunnelURLRe matches Cloudflare quick tunnel URLs in log output.
var tunnelURLRe = regexp.MustCompile(`https://[-a-z0-9]+\.trycloudflare\.com`)

// parseURL scans lines of cloudflared output for a tunnel URL.
// Returns the URL or empty string if not found.
func parseURL(lines []string) string {
	for _, line := range lines {
		if m := tunnelURLRe.FindString(line); m != "" {
			return m
		}
	}
	return ""
}

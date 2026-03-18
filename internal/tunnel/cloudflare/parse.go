package cloudflare

import "regexp"

// tunnelURLRe matches Cloudflare quick tunnel URLs in log output.
var tunnelURLRe = regexp.MustCompile(`https://[-a-z0-9]+\.trycloudflare\.com`)

// parseURL extracts a Cloudflare tunnel URL from a log line.
// Returns the URL or empty string if not found.
func parseURL(line string) string {
	return tunnelURLRe.FindString(line)
}

package cloudflare

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name: "real cloudflared output",
			lines: []string{
				"2026-03-17T14:15:33Z INF Thank you for trying Cloudflare Tunnel.",
				"2026-03-17T14:15:33Z INF Requesting new quick Tunnel on trycloudflare.com...",
				"2026-03-17T14:15:40Z INF +--------------------------------------------------------------------------------------------+",
				"2026-03-17T14:15:40Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |",
				"2026-03-17T14:15:40Z INF |  https://soft-property-mas-trees.trycloudflare.com                                         |",
				"2026-03-17T14:15:40Z INF +--------------------------------------------------------------------------------------------+",
				"2026-03-17T14:15:40Z INF Cannot determine default configuration path.",
			},
			want: "https://soft-property-mas-trees.trycloudflare.com",
		},
		{
			name:  "no url in output",
			lines: []string{"some random log line", "another line"},
			want:  "",
		},
		{
			name: "url with numbers in subdomain",
			lines: []string{
				"2026-03-17T14:15:40Z INF |  https://abc-123-def-456.trycloudflare.com  |",
			},
			want: "https://abc-123-def-456.trycloudflare.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseURL(tt.lines)
			if got != tt.want {
				t.Errorf("parseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

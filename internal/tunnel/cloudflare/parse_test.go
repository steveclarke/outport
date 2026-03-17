package cloudflare

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "url in ascii box line",
			line: "2026-03-17T14:15:40Z INF |  https://soft-property-mas-trees.trycloudflare.com                                         |",
			want: "https://soft-property-mas-trees.trycloudflare.com",
		},
		{
			name: "no url in output",
			line: "some random log line",
			want: "",
		},
		{
			name: "url with numbers in subdomain",
			line: "2026-03-17T14:15:40Z INF |  https://abc-123-def-456.trycloudflare.com  |",
			want: "https://abc-123-def-456.trycloudflare.com",
		},
		{
			name: "non-url log line",
			line: "2026-03-17T14:15:33Z INF Requesting new quick Tunnel on trycloudflare.com...",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseURL(tt.line)
			if got != tt.want {
				t.Errorf("parseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

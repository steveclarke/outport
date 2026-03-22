package urlutil

import "testing"

func TestServiceURL(t *testing.T) {
	tests := []struct {
		name         string
		protocol     string
		hostname     string
		port         int
		httpsEnabled bool
		want         string
	}{
		{"http no hostname", "http", "", 3000, false, "http://localhost:3000"},
		{"https no hostname", "https", "", 8443, false, "https://localhost:8443"},
		{"http non-test hostname", "http", "myapp.localhost", 3000, false, "http://myapp.localhost:3000"},
		{"tcp returns empty", "tcp", "", 5432, false, ""},
		{"empty protocol returns empty", "", "", 6379, false, ""},
		{"http .test with https enabled", "http", "myapp.test", 3000, true, "https://myapp.test"},
		{"http .test without https", "http", "myapp.test", 3000, false, "http://myapp.test"},
		{"https .test with https enabled", "https", "myapp.test", 443, true, "https://myapp.test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceURL(tt.protocol, tt.hostname, tt.port, tt.httpsEnabled)
			if got != tt.want {
				t.Errorf("ServiceURL(%q, %q, %d, %v) = %q, want %q",
					tt.protocol, tt.hostname, tt.port, tt.httpsEnabled, got, tt.want)
			}
		})
	}
}

func TestEffectiveScheme(t *testing.T) {
	tests := []struct {
		name         string
		protocol     string
		hostname     string
		httpsEnabled bool
		want         string
	}{
		{"http .test with https", "http", "myapp.test", true, "https"},
		{"http .test without https", "http", "myapp.test", false, "http"},
		{"https .test with https", "https", "myapp.test", true, "https"},
		{"tcp .test with https", "tcp", "myapp.test", true, "tcp"},
		{"http non-test with https", "http", "myapp.localhost", true, "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveScheme(tt.protocol, tt.hostname, tt.httpsEnabled)
			if got != tt.want {
				t.Errorf("EffectiveScheme(%q, %q, %v) = %q, want %q",
					tt.protocol, tt.hostname, tt.httpsEnabled, got, tt.want)
			}
		})
	}
}

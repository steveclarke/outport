package urlutil

import "testing"

func TestServiceURL(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		port         int
		httpsEnabled bool
		want         string
	}{
		{"no hostname returns empty", "", 3000, false, ""},
		{"non-test hostname", "myapp.localhost", 3000, false, "http://myapp.localhost:3000"},
		{".test with https enabled", "myapp.test", 3000, true, "https://myapp.test"},
		{".test without https", "myapp.test", 3000, false, "http://myapp.test"},
		{"subdomain .test with https", "api.myapp.test", 443, true, "https://api.myapp.test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServiceURL(tt.hostname, tt.port, tt.httpsEnabled)
			if got != tt.want {
				t.Errorf("ServiceURL(%q, %d, %v) = %q, want %q",
					tt.hostname, tt.port, tt.httpsEnabled, got, tt.want)
			}
		})
	}
}

func TestEffectiveScheme(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		httpsEnabled bool
		want         string
	}{
		{".test with https", "myapp.test", true, "https"},
		{".test without https", "myapp.test", false, "http"},
		{"non-test with https", "myapp.localhost", true, "http"},
		{"empty hostname", "", false, "http"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveScheme(tt.hostname, tt.httpsEnabled)
			if got != tt.want {
				t.Errorf("EffectiveScheme(%q, %v) = %q, want %q",
					tt.hostname, tt.httpsEnabled, got, tt.want)
			}
		})
	}
}

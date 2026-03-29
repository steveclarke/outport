//go:build linux

package doctor

import "testing"

func TestJoinNames(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"Chrome"}, "Chrome"},
		{[]string{"Chrome", "Firefox"}, "Chrome and Firefox"},
		{[]string{"Chrome", "Firefox", "Snap Firefox"}, "Chrome, Firefox, and Snap Firefox"},
		{[]string{"A", "B", "C", "D"}, "A, B, C, and D"},
	}
	for _, tt := range tests {
		if got := joinNames(tt.names); got != tt.want {
			t.Errorf("joinNames(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

func TestLinuxBrowserTrustChecks(t *testing.T) {
	checks := linuxBrowserTrustChecks()
	if len(checks) != 2 {
		t.Fatalf("expected 2 browser trust checks, got %d", len(checks))
	}

	expected := []struct {
		name     string
		category string
	}{
		{"certutil installed", "TLS"},
		{"Browser CA trust", "TLS"},
	}

	for i, want := range expected {
		if checks[i].Name != want.name {
			t.Errorf("check %d: name = %q, want %q", i, checks[i].Name, want.name)
		}
		if checks[i].Category != want.category {
			t.Errorf("check %d: category = %q, want %q", i, checks[i].Category, want.category)
		}
	}
}

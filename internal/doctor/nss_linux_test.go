//go:build linux

package doctor

import "testing"

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

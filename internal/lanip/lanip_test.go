package lanip

import (
	"testing"
)

func TestDetect_AutoReturnsValidIP(t *testing.T) {
	ip, err := Detect("")
	if err != nil {
		t.Skipf("no LAN interface available: %v", err)
	}
	if ip.IsLoopback() {
		t.Errorf("returned loopback address: %v", ip)
	}
	if ip.To4() == nil {
		t.Errorf("expected IPv4, got: %v", ip)
	}
}

func TestDetect_InvalidInterface(t *testing.T) {
	_, err := Detect("nonexistent99")
	if err == nil {
		t.Error("expected error for nonexistent interface")
	}
}

func TestIsVirtual(t *testing.T) {
	cases := []struct {
		name    string
		virtual bool
	}{
		{"en0", false},
		{"en1", false},
		{"eth0", false},
		{"utun3", true},
		{"bridge0", true},
		{"veth12345", true},
		{"docker0", true},
		{"vmnet1", true},
	}
	for _, tc := range cases {
		if got := isVirtual(tc.name); got != tc.virtual {
			t.Errorf("isVirtual(%q) = %v, want %v", tc.name, got, tc.virtual)
		}
	}
}

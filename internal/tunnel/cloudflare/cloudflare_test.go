package cloudflare

import (
	"os"
	"testing"
)

func TestCheckAvailable_NotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	p := New()
	err := p.CheckAvailable()
	if err == nil {
		t.Fatal("expected error when cloudflared not in PATH")
	}
	if got := err.Error(); got != "cloudflared not found. Install with: brew install cloudflared" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestCheckAvailable_Installed(t *testing.T) {
	dir := t.TempDir()
	fake := dir + "/cloudflared"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	p := New()
	if err := p.CheckAvailable(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestName(t *testing.T) {
	p := New()
	if got := p.Name(); got != "cloudflare" {
		t.Errorf("Name() = %q, want %q", got, "cloudflare")
	}
}

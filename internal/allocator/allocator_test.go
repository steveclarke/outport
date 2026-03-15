package allocator

import (
	"testing"
)

func TestHashPort_Deterministic(t *testing.T) {
	port1 := HashPort("myapp", "main", "web")
	port2 := HashPort("myapp", "main", "web")
	if port1 != port2 {
		t.Errorf("not deterministic: %d != %d", port1, port2)
	}
}

func TestHashPort_InRange(t *testing.T) {
	cases := []struct {
		project, instance, service string
	}{
		{"myapp", "main", "web"},
		{"myapp", "main", "postgres"},
		{"myapp", "feature-xyz", "web"},
		{"other-project", "main", "redis"},
	}
	for _, tc := range cases {
		port := HashPort(tc.project, tc.instance, tc.service)
		if port < MinPort || port > MaxPort {
			t.Errorf("HashPort(%q, %q, %q) = %d, outside range [%d, %d]",
				tc.project, tc.instance, tc.service, port, MinPort, MaxPort)
		}
	}
}

func TestHashPort_DifferentInputsDifferentPorts(t *testing.T) {
	p1 := HashPort("myapp", "main", "web")
	p2 := HashPort("myapp", "main", "postgres")
	p3 := HashPort("myapp", "feature-xyz", "web")
	if p1 == p2 {
		t.Errorf("web and postgres got same port: %d", p1)
	}
	if p1 == p3 {
		t.Errorf("main/web and feature-xyz/web got same port: %d", p1)
	}
}

func TestAllocate_NoCollisions(t *testing.T) {
	port, err := Allocate("myapp", "main", "web", 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := HashPort("myapp", "main", "web")
	if port != expected {
		t.Errorf("Allocate = %d, want %d (no collisions, should match hash)", port, expected)
	}
}

func TestAllocate_WithCollision(t *testing.T) {
	idealPort := HashPort("myapp", "main", "web")
	usedPorts := map[int]bool{idealPort: true}

	port, err := Allocate("myapp", "main", "web", 0, usedPorts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == idealPort {
		t.Errorf("should have avoided collision with port %d", idealPort)
	}
	if port < MinPort || port > MaxPort {
		t.Errorf("collision-resolved port %d outside range", port)
	}
}

func TestAllocate_WithMultipleCollisions(t *testing.T) {
	idealPort := HashPort("myapp", "main", "web")
	usedPorts := make(map[int]bool)
	for i := 0; i < 3; i++ {
		p := idealPort + i
		if p > MaxPort {
			p = MinPort + (p - MaxPort - 1)
		}
		usedPorts[p] = true
	}

	port, err := Allocate("myapp", "main", "web", 0, usedPorts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usedPorts[port] {
		t.Errorf("port %d is still in used set", port)
	}
	if port < MinPort || port > MaxPort {
		t.Errorf("port %d outside range", port)
	}
}

func TestAllocate_PreferredPortAvailable(t *testing.T) {
	port, err := Allocate("myapp", "main", "web", 3000, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3000 {
		t.Errorf("port = %d, want 3000 (preferred port was available)", port)
	}
}

func TestAllocate_PreferredPortTaken(t *testing.T) {
	usedPorts := map[int]bool{3000: true}
	port, err := Allocate("myapp", "main", "web", 3000, usedPorts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 3000 {
		t.Error("should not have used taken preferred port")
	}
	if port < MinPort || port > MaxPort {
		t.Errorf("port %d outside range", port)
	}
}

func TestAllocate_PreferredPortZero(t *testing.T) {
	port, err := Allocate("myapp", "main", "web", 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := HashPort("myapp", "main", "web")
	if port != expected {
		t.Errorf("port = %d, want %d (no preferred, should use hash)", port, expected)
	}
}

func TestReservedPortPreferredFallsBack(t *testing.T) {
	port, err := Allocate("proj", "inst", "svc", ReservedDNSPort, map[int]bool{})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if port == ReservedDNSPort {
		t.Fatalf("should not allocate reserved port even when preferred")
	}
}

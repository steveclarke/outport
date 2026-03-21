package dashboard

import (
	"testing"
)

func TestHealthCheckerDetectsChanges(t *testing.T) {
	prev := map[int]bool{10001: true, 10002: false}
	curr := map[int]bool{10001: true, 10002: true}

	changes := detectChanges(prev, curr)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[10002] != true {
		t.Error("expected port 10002 changed to true")
	}
}

func TestHealthCheckerNoChanges(t *testing.T) {
	prev := map[int]bool{10001: true, 10002: false}
	curr := map[int]bool{10001: true, 10002: false}

	changes := detectChanges(prev, curr)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestHealthCheckerNewPort(t *testing.T) {
	prev := map[int]bool{10001: true}
	curr := map[int]bool{10001: true, 10002: false}

	changes := detectChanges(prev, curr)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if _, ok := changes[10002]; !ok {
		t.Error("expected port 10002 in changes")
	}
}

func TestHealthCheckerRemovedPort(t *testing.T) {
	prev := map[int]bool{10001: true, 10002: true}
	curr := map[int]bool{10001: true}

	changes := detectChanges(prev, curr)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[10002] != false {
		t.Error("expected port 10002 changed to false (removed)")
	}
}

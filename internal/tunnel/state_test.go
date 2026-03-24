package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteState_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tunnels.json")

	tunnels := map[string]string{
		"rails": "https://abc-123.trycloudflare.com",
		"vite":  "https://def-456.trycloudflare.com",
	}
	err := WriteState(path, "myapp/main", tunnels)
	if err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}
}

func TestReadState_ReturnsWrittenData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tunnels.json")

	tunnels := map[string]string{
		"rails": "https://abc-123.trycloudflare.com",
	}
	if err := WriteState(path, "myapp/main", tunnels); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	state, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
		return
	}
	url, ok := state.Tunnels["myapp/main"]["rails"]
	if !ok || url != "https://abc-123.trycloudflare.com" {
		t.Errorf("unexpected tunnel URL: %q", url)
	}
}

func TestReadState_MissingFile(t *testing.T) {
	state, err := ReadState("/nonexistent/tunnels.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for missing file")
	}
}

func TestRemoveState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tunnels.json")

	tunnels := map[string]string{"rails": "https://example.com"}
	if err := WriteState(path, "myapp/main", tunnels); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	RemoveState(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("state file should have been removed")
	}
}

func TestReadState_StalePID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tunnels.json")

	// Write a state file with PID 0 (never valid)
	data := []byte(`{"pid":0,"tunnels":{"myapp/main":{"rails":"https://example.com"}}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	state, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if state != nil {
		t.Error("expected nil for stale PID")
	}
}

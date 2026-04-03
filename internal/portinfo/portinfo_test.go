package portinfo

import (
	"os"
	"testing"
	"time"
)

// fakeLister returns canned ProcessInfo for testing.
type fakeLister struct {
	processes []ProcessInfo
	err       error
}

func (f *fakeLister) ListProcesses() ([]ProcessInfo, error) {
	return f.processes, f.err
}

func TestScan(t *testing.T) {
	lister := &fakeLister{
		processes: []ProcessInfo{
			{PID: 48291, PPID: 1, Name: "node", Command: "node .next/standalone/server.js", Port: 13542, RSS: 145981440, Elapsed: 2*time.Hour + 14*time.Minute, State: "S"},
			{PID: 51002, PPID: 1042, Name: "ruby", Command: "ruby bin/rails server -p 3000", Port: 3000, RSS: 100663296, Elapsed: 23*time.Hour + 30*time.Minute, State: "S"},
		},
	}

	results, err := Scan(lister)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Results are sorted by port, so ruby (3000) comes first
	ruby := results[0]
	if ruby.Port != 3000 {
		t.Errorf("first result port = %d, want 3000", ruby.Port)
	}
	if ruby.PID != 51002 {
		t.Errorf("ruby PID = %d, want 51002", ruby.PID)
	}
	if ruby.IsOrphan {
		t.Error("ruby should not be orphan (ppid=1042)")
	}

	node := results[1]
	if node.Port != 13542 {
		t.Errorf("second result port = %d, want 13542", node.Port)
	}
	if node.PID != 48291 {
		t.Errorf("node PID = %d, want 48291", node.PID)
	}
	if node.PPID != 1 {
		t.Errorf("node PPID = %d, want 1", node.PPID)
	}
	if node.RSS != 145981440 {
		t.Errorf("node RSS = %d, want 145981440", node.RSS)
	}
	if !node.IsOrphan {
		t.Error("node should be orphan (ppid=1, dev process)")
	}
	if node.Command != "node .next/standalone/server.js" {
		t.Errorf("node command = %q", node.Command)
	}
}

func TestScanPorts(t *testing.T) {
	lister := &fakeLister{
		processes: []ProcessInfo{
			{PID: 48291, PPID: 1, Name: "node", Port: 13542, State: "S"},
			{PID: 51002, PPID: 1042, Name: "ruby", Port: 3000, State: "S"},
			{PID: 412, PPID: 1, Name: "postgres", Port: 5432, State: "S"},
		},
	}

	results, err := ScanPorts([]int{13542}, lister)
	if err != nil {
		t.Fatalf("ScanPorts() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Port != 13542 {
		t.Errorf("port = %d, want 13542", results[0].Port)
	}
}

func TestScan_Empty(t *testing.T) {
	lister := &fakeLister{processes: nil}

	results, err := Scan(lister)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestKill_RefuseProtectedPIDs(t *testing.T) {
	tests := []struct {
		name string
		pid  int
	}{
		{"PID 0", 0},
		{"PID 1", 1},
		{"negative PID", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Kill(tt.pid)
			if err == nil {
				t.Errorf("Kill(%d) should return error", tt.pid)
			}
		})
	}
}

func TestKill_RefuseOwnProcess(t *testing.T) {
	err := Kill(os.Getpid())
	if err == nil {
		t.Error("Kill(own PID) should return error")
	}
}

func TestUptimeSeconds(t *testing.T) {
	p := ProcessInfo{Elapsed: 2*time.Hour + 14*time.Minute + 30*time.Second}
	got := p.UptimeSeconds()
	if got != 8070 {
		t.Errorf("UptimeSeconds() = %d, want 8070", got)
	}
}

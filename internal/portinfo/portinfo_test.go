// internal/portinfo/portinfo_test.go
package portinfo

import (
	"os"
	"testing"
)

type fakeScanner struct {
	listeningOutput string
	listeningErr    error
	psOutput        string
	psErr           error
	cwdOutput       string
	cwdErr          error
}

func (f *fakeScanner) ListeningPorts() (string, error) {
	return f.listeningOutput, f.listeningErr
}

func (f *fakeScanner) ProcessInfo(pids []int) (string, error) {
	return f.psOutput, f.psErr
}

func (f *fakeScanner) WorkingDirs(pids []int) (string, error) {
	return f.cwdOutput, f.cwdErr
}

func TestScan(t *testing.T) {
	scanner := &fakeScanner{
		listeningOutput: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
`,
		psOutput: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
51002  1042 S   98304 Wed Mar 26 14:30:00 2026 ruby bin/rails server -p 3000
`,
		cwdOutput: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /tmp/fake-myapp
ruby    51002 steve  cwd    DIR    1,4      320 5678 /tmp/fake-railsapp
`,
	}

	results, err := Scan(scanner)
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
	if node.RSS != 142560*1024 {
		t.Errorf("node RSS = %d, want %d", node.RSS, 142560*1024)
	}
	if !node.IsOrphan {
		t.Error("node should be orphan (ppid=1, dev process)")
	}
	if node.Command != "node .next/standalone/server.js" {
		t.Errorf("node command = %q", node.Command)
	}
}

func TestScanPorts(t *testing.T) {
	scanner := &fakeScanner{
		listeningOutput: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
postgres    412  steve    5u  IPv4 0xaabbccddee      0t0  TCP 127.0.0.1:5432 (LISTEN)
`,
		psOutput: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
`,
		cwdOutput: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /tmp/fake-myapp
`,
	}

	results, err := ScanPorts([]int{13542}, scanner)
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

func TestScan_EmptyOutput(t *testing.T) {
	scanner := &fakeScanner{listeningOutput: ""}

	results, err := Scan(scanner)
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

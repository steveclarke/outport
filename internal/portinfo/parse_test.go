package portinfo

import (
	"testing"
	"time"
)


func TestParseLsofListening(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []lsofEntry
		wantErr bool
	}{
		{
			name: "typical macOS output",
			input: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
ruby      51002  steve   10u  IPv6 0x0987654321      0t0  TCP *:3000 (LISTEN)
postgres    412  steve    5u  IPv4 0xaabbccddee      0t0  TCP 127.0.0.1:5432 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 48291, ProcessName: "node", Port: 13542},
				{PID: 51002, ProcessName: "ruby", Port: 3000},
				{PID: 412, ProcessName: "postgres", Port: 5432},
			},
		},
		{
			name: "IPv6 with bracket notation",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    12345 steve   22u  IPv6 0x1234      0t0  TCP [::1]:8080 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 12345, ProcessName: "node", Port: 8080},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "header only",
			input: "COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME\n",
			want:  nil,
		},
		{
			name: "wildcard listen address",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    99999 steve   22u  IPv4 0x1234      0t0  TCP *:4000 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 99999, ProcessName: "node", Port: 4000},
			},
		},
		{
			name: "duplicate port different PIDs keeps both",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    100 steve   22u  IPv4 0x1234      0t0  TCP *:3000 (LISTEN)
node    200 steve   23u  IPv4 0x5678      0t0  TCP *:3000 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 100, ProcessName: "node", Port: 3000},
				{PID: 200, ProcessName: "node", Port: 3000},
			},
		},
		{
			name: "malformed line skipped",
			input: `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
this is not valid lsof output
node      48291  steve   22u  IPv4 0x1234567890      0t0  TCP 127.0.0.1:13542 (LISTEN)
`,
			want: []lsofEntry{
				{PID: 48291, ProcessName: "node", Port: 13542},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsofListening(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLsofListening() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParsePsOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[int]psEntry
	}{
		{
			name: "typical output with etime",
			input: `48291     1 S  142560 02:14:30 node .next/standalone/server.js
51002  1042 S   98304 23:30:00 ruby bin/rails server -p 3000
  412     1 Ss  25600 10-08:00:00 /usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Elapsed: 2*time.Hour + 14*time.Minute + 30*time.Second, Command: "node .next/standalone/server.js"},
				51002: {PID: 51002, PPID: 1042, State: "S", RSS: 98304, Elapsed: 23*time.Hour + 30*time.Minute, Command: "ruby bin/rails server -p 3000"},
				412:   {PID: 412, PPID: 1, State: "Ss", RSS: 25600, Elapsed: 10*24*time.Hour + 8*time.Hour, Command: "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main"},
			},
		},
		{
			name: "short etime mm:ss",
			input: `99999     1 S  1024 00:45 node server.js
`,
			want: map[int]psEntry{
				99999: {PID: 99999, PPID: 1, State: "S", RSS: 1024, Elapsed: 45 * time.Second, Command: "node server.js"},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int]psEntry{},
		},
		{
			name: "malformed line skipped",
			input: `not valid ps output
48291     1 S  142560 14:30 node server.js
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Elapsed: 14*time.Minute + 30*time.Second, Command: "node server.js"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePsOutput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for pid, wantEntry := range tt.want {
				gotEntry, ok := got[pid]
				if !ok {
					t.Errorf("missing PID %d", pid)
					continue
				}
				if gotEntry.PID != wantEntry.PID || gotEntry.PPID != wantEntry.PPID ||
					gotEntry.State != wantEntry.State || gotEntry.RSS != wantEntry.RSS ||
					gotEntry.Command != wantEntry.Command {
					t.Errorf("PID %d: got %+v, want %+v", pid, gotEntry, wantEntry)
				}
				if gotEntry.Elapsed != wantEntry.Elapsed {
					t.Errorf("PID %d: Elapsed = %v, want %v", pid, gotEntry.Elapsed, wantEntry.Elapsed)
				}
			}
		})
	}
}

func TestParseLsofCwd(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[int]string
	}{
		{
			name: "typical output",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /Users/steve/src/myapp
ruby    51002 steve  cwd    DIR    1,4      320 5678 /Users/steve/src/railsapp
`,
			want: map[int]string{
				48291: "/Users/steve/src/myapp",
				51002: "/Users/steve/src/railsapp",
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int]string{},
		},
		{
			name: "path with spaces",
			input: `COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node    48291 steve  cwd    DIR    1,4      640 1234 /Users/steve/my project/app
`,
			want: map[int]string{
				48291: "/Users/steve/my project/app",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLsofCwd(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for pid, wantCwd := range tt.want {
				if got[pid] != wantCwd {
					t.Errorf("PID %d: got %q, want %q", pid, got[pid], wantCwd)
				}
			}
		})
	}
}

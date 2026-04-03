package portinfo

import (
	"testing"
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
			name: "typical output",
			input: `48291     1 S  142560 Thu Mar 27 09:15:00 2026 node .next/standalone/server.js
51002  1042 S   98304 Wed Mar 26 14:30:00 2026 ruby bin/rails server -p 3000
  412     1 Ss  25600 Mon Mar 24 08:00:00 2026 /usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Command: "node .next/standalone/server.js"},
				51002: {PID: 51002, PPID: 1042, State: "S", RSS: 98304, Command: "ruby bin/rails server -p 3000"},
				412:   {PID: 412, PPID: 1, State: "Ss", RSS: 25600, Command: "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main"},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int]psEntry{},
		},
		{
			name: "EU locale lstart format",
			input: `73959 73958 S  279200 Tue 31 Mar 22:38:00 2026 /Applications/Docker.app/Contents/MacOS/com.docker.backend services
`,
			want: map[int]psEntry{
				73959: {PID: 73959, PPID: 73958, State: "S", RSS: 279200, Command: "/Applications/Docker.app/Contents/MacOS/com.docker.backend services"},
			},
		},
		{
			name: "malformed line skipped",
			input: `not valid ps output
48291     1 S  142560 Thu Mar 27 09:15:00 2026 node server.js
`,
			want: map[int]psEntry{
				48291: {PID: 48291, PPID: 1, State: "S", RSS: 142560, Command: "node server.js"},
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
				if gotEntry.StartTime.IsZero() {
					t.Errorf("PID %d: StartTime is zero", pid)
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

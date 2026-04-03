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

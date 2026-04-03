package cmd

import (
	"testing"
	"time"

	"github.com/steveclarke/outport/internal/portinfo"
)

func TestPortsCmd_Registered(t *testing.T) {
	// Verify the command is registered on rootCmd
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "ports" {
			found = true
			if cmd.GroupID != "project" {
				t.Errorf("ports command GroupID = %q, want %q", cmd.GroupID, "project")
			}
			break
		}
	}
	if !found {
		t.Fatal("ports command not registered on rootCmd")
	}
}

func TestPortsCmd_HasAllFlag(t *testing.T) {
	cmd := portsCmd
	flag := cmd.Flags().Lookup("all")
	if flag == nil {
		t.Fatal("ports command missing --all flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--all default = %q, want %q", flag.DefValue, "false")
	}
}

func TestPortsCmd_AcceptsJsonFlag(t *testing.T) {
	// --json is a persistent flag on rootCmd, should be inherited
	flag := rootCmd.PersistentFlags().Lookup("json")
	if flag == nil {
		t.Fatal("root command missing --json persistent flag")
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, ""},
		{"negative", -1, ""},
		{"kilobytes", 512 * 1024, "512 KB"},
		{"megabytes", 142 * 1024 * 1024, "142 MB"},
		{"gigabytes", int64(1.5 * 1024 * 1024 * 1024), "1.5 GB"},
		{"small megabytes", 5 * 1024 * 1024, "5 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMemory(tt.bytes)
			if got != tt.want {
				t.Errorf("formatMemory(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, ""},
		{"negative", -1 * time.Second, ""},
		{"seconds", 30 * time.Second, "<1m"},
		{"minutes", 45 * time.Minute, "45m"},
		{"hours and minutes", 2*time.Hour + 14*time.Minute, "2h 14m"},
		{"days", 3*24*time.Hour + 5*time.Hour, "3d 5h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.d)
			if got != tt.want {
				t.Errorf("formatUptime(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestIndexByPort(t *testing.T) {
	procs := []portinfo.ProcessInfo{
		{PID: 100, Port: 3000},
		{PID: 200, Port: 5432},
	}
	m := indexByPort(procs)
	if len(m) != 2 {
		t.Fatalf("got %d entries, want 2", len(m))
	}
	if m[3000].PID != 100 {
		t.Errorf("port 3000 PID = %d, want 100", m[3000].PID)
	}
	if m[5432].PID != 200 {
		t.Errorf("port 5432 PID = %d, want 200", m[5432].PID)
	}
}

// fakePortLister implements portinfo.Lister for testing — returns no processes.
type fakePortLister struct{}

func (f fakePortLister) ListProcesses() ([]portinfo.ProcessInfo, error) { return nil, nil }

func TestPortsProject_JSON(t *testing.T) {
	setupProject(t, testConfigWithHTTP)

	// Replace the lister with a fake that returns no processes
	origLister := portLister
	portLister = fakePortLister{}
	t.Cleanup(func() { portLister = origLister })

	// First register the project
	executeCmd(t, "up")

	// Run ports --json --down (need --down since fake lister returns no processes)
	jsonFlag = true
	portsDownFlag = true
	t.Cleanup(func() { jsonFlag = false; portsDownFlag = false })
	output := executeCmd(t, "ports", "--json", "--down")

	var result portsProjectJSON
	unwrapJSON(t, output, &result)

	if result.Project != "testapp" {
		t.Errorf("project = %q, want %q", result.Project, "testapp")
	}
	if result.Instance != "main" {
		t.Errorf("instance = %q, want %q", result.Instance, "main")
	}
	if len(result.Ports) != 3 {
		t.Fatalf("ports count = %d, want 3", len(result.Ports))
	}

	// All ports should be down since we're using a fake lister
	for _, entry := range result.Ports {
		if entry.Up {
			t.Errorf("port %d should be down with fake lister", entry.Port)
		}
		if entry.Service == "" {
			t.Error("service name should not be empty")
		}
	}
}

func TestPortsAll_JSON(t *testing.T) {
	setupProject(t, testConfig)

	origLister := portLister
	portLister = fakePortLister{}
	t.Cleanup(func() { portLister = origLister })

	executeCmd(t, "up")

	jsonFlag = true
	portsAllFlag = true
	portsDownFlag = true
	t.Cleanup(func() {
		jsonFlag = false
		portsAllFlag = false
		portsDownFlag = false
	})
	output := executeCmd(t, "ports", "--all", "--json", "--down")

	var result portsAllJSON
	unwrapJSON(t, output, &result)

	// With fake lister + --down, all managed ports appear but are down
	for _, entry := range result.Managed {
		if entry.Up {
			t.Errorf("managed port %d should be down with fake lister", entry.Port)
		}
	}
}

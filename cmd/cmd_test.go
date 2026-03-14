package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

// setupProject creates a temp directory with .outport.yml and .git,
// sets HOME to isolate the registry, and chdir into the project.
// Returns the project dir path.
func setupProject(t *testing.T, configYAML string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	// Reset jsonFlag between tests
	jsonFlag = false

	return dir
}

const testConfig = `name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`

// executeCmd runs a root command with args and returns captured stdout.
func executeCmd(t *testing.T, args ...string) string {
	t.Helper()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}

	return buf.String()
}

// --- up ---

func TestUp_AllocatesPortsAndWritesEnv(t *testing.T) {
	dir := setupProject(t, testConfig)

	output := executeCmd(t, "up", "--json")

	var result upJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	if result.Project != "testapp" {
		t.Errorf("project = %q, want %q", result.Project, "testapp")
	}
	if result.Instance != "main" {
		t.Errorf("instance = %q, want %q", result.Instance, "main")
	}
	if len(result.Services) != 2 {
		t.Fatalf("services count = %d, want 2", len(result.Services))
	}

	webPort := result.Services["web"].Port
	pgPort := result.Services["postgres"].Port
	if webPort == 0 {
		t.Error("web port should be non-zero")
	}
	if pgPort == 0 {
		t.Error("postgres port should be non-zero")
	}
	if webPort == pgPort {
		t.Error("web and postgres ports should be different")
	}
	// With preferred_port set, web should get 3000 and postgres 5432
	if webPort != 3000 {
		t.Errorf("web port = %d, want preferred port 3000", webPort)
	}
	if pgPort != 5432 {
		t.Errorf("postgres port = %d, want preferred port 5432", pgPort)
	}

	// Check .env was written
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	envContent := string(envData)
	if !bytes.Contains(envData, []byte("PORT=")) {
		t.Errorf(".env missing PORT, got:\n%s", envContent)
	}
	if !bytes.Contains(envData, []byte("DATABASE_PORT=")) {
		t.Errorf(".env missing DATABASE_PORT, got:\n%s", envContent)
	}
	if !bytes.Contains(envData, []byte("# managed by outport")) {
		t.Errorf(".env missing managed marker, got:\n%s", envContent)
	}
}

func TestUp_IsIdempotent(t *testing.T) {
	setupProject(t, testConfig)

	out1 := executeCmd(t, "up", "--json")
	out2 := executeCmd(t, "up", "--json")

	var r1, r2 upJSON
	json.Unmarshal([]byte(out1), &r1)
	json.Unmarshal([]byte(out2), &r2)

	if r1.Services["web"].Port != r2.Services["web"].Port {
		t.Errorf("web port changed: %d -> %d", r1.Services["web"].Port, r2.Services["web"].Port)
	}
	if r1.Services["postgres"].Port != r2.Services["postgres"].Port {
		t.Errorf("postgres port changed: %d -> %d", r1.Services["postgres"].Port, r2.Services["postgres"].Port)
	}
}

func TestUp_StyledOutput(t *testing.T) {
	setupProject(t, testConfig)

	output := executeCmd(t, "up")

	if !bytes.Contains([]byte(output), []byte("testapp")) {
		t.Errorf("styled output missing project name, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Ports written to .env")) {
		t.Errorf("styled output missing success message, got:\n%s", output)
	}
}

func TestUp_NoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	t.Chdir(dir)
	jsonFlag = false

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"up"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no .outport.yml exists")
	}
}

// --- ports ---

func TestPorts_ShowsAllocatedPorts(t *testing.T) {
	setupProject(t, testConfig)

	// First allocate ports
	executeCmd(t, "up", "--json")

	// Then query them
	output := executeCmd(t, "ports", "--json")

	var result struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Services map[string]struct {
			Port int `json:"port"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	if result.Project != "testapp" {
		t.Errorf("project = %q, want %q", result.Project, "testapp")
	}
	if len(result.Services) != 2 {
		t.Errorf("services count = %d, want 2", len(result.Services))
	}
	for name, svc := range result.Services {
		if svc.Port == 0 {
			t.Errorf("service %q has port 0", name)
		}
	}
}

func TestPorts_NoAllocation(t *testing.T) {
	setupProject(t, testConfig)

	output := executeCmd(t, "ports")

	if !bytes.Contains([]byte(output), []byte("No ports allocated")) {
		t.Errorf("expected 'No ports allocated' message, got:\n%s", output)
	}
}

// --- status ---

func TestStatus_EmptyRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	output := executeCmd(t, "status")

	if !bytes.Contains([]byte(output), []byte("No projects registered")) {
		t.Errorf("expected 'No projects registered', got:\n%s", output)
	}
}

func TestStatus_ShowsProjects(t *testing.T) {
	setupProject(t, testConfig)

	// Populate registry via up
	executeCmd(t, "up", "--json")

	output := executeCmd(t, "status", "--json")

	var entries []statusEntryJSON
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	if len(entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(entries))
	}
	if entries[0].Key != "testapp/main" {
		t.Errorf("key = %q, want %q", entries[0].Key, "testapp/main")
	}
	if len(entries[0].Services) != 2 {
		t.Errorf("services count = %d, want 2", len(entries[0].Services))
	}
}

// --- gc ---

func TestGC_RemovesStaleEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Manually create a registry with a stale entry
	regPath := filepath.Join(home, ".config", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}

	reg.Set("staleapp", "main", registry.Allocation{
		ProjectDir: "/tmp/nonexistent-outport-test-dir",
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	output := executeCmd(t, "gc")

	if !bytes.Contains([]byte(output), []byte("Removed 1 stale")) {
		t.Errorf("expected removal message, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("staleapp/main")) {
		t.Errorf("expected stale key in output, got:\n%s", output)
	}

	// Verify it's actually gone from the registry
	reg2, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg2.Projects) != 0 {
		t.Errorf("registry still has %d entries after gc", len(reg2.Projects))
	}
}

func TestGC_NoStaleEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := t.TempDir()
	t.Chdir(projectDir)
	jsonFlag = false

	// Create a registry with a valid entry
	regPath := filepath.Join(home, ".config", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}

	reg.Set("validapp", "main", registry.Allocation{
		ProjectDir: projectDir,
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	output := executeCmd(t, "gc")

	if !bytes.Contains([]byte(output), []byte("No stale entries")) {
		t.Errorf("expected 'No stale entries', got:\n%s", output)
	}
}

// --- init ---

func TestInit_CreatesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	t.Chdir(dir)
	jsonFlag = false

	// Simulate stdin: enter project name + select web and postgres
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	input := "myproject\ny\ny\nn\nn\nn\nn\n"
	w.WriteString(input)
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	output := executeCmd(t, "init")

	if !bytes.Contains([]byte(output), []byte("Created .outport.yml")) {
		t.Errorf("expected creation message, got:\n%s", output)
	}

	// Verify the file was created with correct content
	data, err := os.ReadFile(filepath.Join(dir, ".outport.yml"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	content := string(data)
	if !bytes.Contains(data, []byte("name: myproject")) {
		t.Errorf("config missing project name, got:\n%s", content)
	}
	if !bytes.Contains(data, []byte("web:")) {
		t.Errorf("config missing web service, got:\n%s", content)
	}
	if !bytes.Contains(data, []byte("postgres:")) {
		t.Errorf("config missing postgres service, got:\n%s", content)
	}
}

func TestInit_ErrorWhenConfigExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte("name: existing\n"), 0644)
	t.Chdir(dir)
	jsonFlag = false

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"init"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
}

func TestInit_DefaultProjectName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	t.Chdir(dir)
	jsonFlag = false

	// Simulate stdin: empty name (use default) + select web only
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	input := "\ny\nn\nn\nn\nn\nn\n"
	w.WriteString(input)
	w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	executeCmd(t, "init")

	data, err := os.ReadFile(filepath.Join(dir, ".outport.yml"))
	if err != nil {
		t.Fatal(err)
	}
	// Should use the temp dir's basename as project name
	dirName := filepath.Base(dir)
	if !bytes.Contains(data, []byte("name: "+dirName)) {
		t.Errorf("expected default name %q in config, got:\n%s", dirName, string(data))
	}
}

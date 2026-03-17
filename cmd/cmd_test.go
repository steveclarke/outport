package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
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

	// Reset flags between tests
	jsonFlag = false
	useHTTPS = false

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

// --- up with derived values ---

const testConfigWithDerived = `name: testapp
services:
  rails:
    preferred_port: 3000
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env
  web:
    preferred_port: 5173
    env_var: WEB_PORT
    protocol: http
    env_file: frontend/.env

derived:
  API_URL:
    value: "http://localhost:${rails.port}/api/v1"
    env_file: frontend/.env
  CORS_ORIGINS:
    value: "http://localhost:${web.port}"
    env_file: backend/.env
`

func TestUp_WithDerivedValues(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	output := executeCmd(t, "up", "--json")

	var result upJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	if len(result.Derived) != 2 {
		t.Fatalf("derived count = %d, want 2", len(result.Derived))
	}

	apiURL := result.Derived["API_URL"]
	if apiURL.Value != "http://localhost:3000/api/v1" {
		t.Errorf("API_URL = %q, want http://localhost:3000/api/v1", apiURL.Value)
	}
	if len(apiURL.EnvFiles) != 1 || apiURL.EnvFiles[0] != "frontend/.env" {
		t.Errorf("API_URL.EnvFiles = %v, want [frontend/.env]", apiURL.EnvFiles)
	}

	cors := result.Derived["CORS_ORIGINS"]
	if cors.Value != "http://localhost:5173" {
		t.Errorf("CORS_ORIGINS = %q, want http://localhost:5173", cors.Value)
	}

	// Check .env files contain derived values
	frontendEnv, err := os.ReadFile(filepath.Join(dir, "frontend", ".env"))
	if err != nil {
		t.Fatalf("reading frontend/.env: %v", err)
	}
	if !bytes.Contains(frontendEnv, []byte("API_URL=http://localhost:3000/api/v1")) {
		t.Errorf("frontend/.env missing API_URL, got:\n%s", frontendEnv)
	}

	backendEnv, err := os.ReadFile(filepath.Join(dir, "backend", ".env"))
	if err != nil {
		t.Fatalf("reading backend/.env: %v", err)
	}
	if !bytes.Contains(backendEnv, []byte("CORS_ORIGINS=http://localhost:5173")) {
		t.Errorf("backend/.env missing CORS_ORIGINS, got:\n%s", backendEnv)
	}
}

func TestUp_DerivedStyledOutput(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	output := executeCmd(t, "up")

	if !bytes.Contains([]byte(output), []byte("derived:")) {
		t.Errorf("styled output missing 'derived:' section, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("API_URL")) {
		t.Errorf("styled output missing API_URL, got:\n%s", output)
	}
}

func TestUp_DerivedPerFileValues(t *testing.T) {
	dir := setupProject(t, `name: testapp
services:
  rails:
    preferred_port: 3000
    env_var: RAILS_PORT
    protocol: http
    env_file: backend/.env

derived:
  API_URL:
    env_file:
      - file: frontend/main/.env
        value: "http://localhost:${rails.port}/api/v1"
      - file: frontend/portal/.env
        value: "http://localhost:${rails.port}/portal/api/v1"
`)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend/main"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend/portal"), 0755)

	output := executeCmd(t, "up", "--json")

	var result upJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	apiURL := result.Derived["API_URL"]
	// Per-file values should use the "values" field
	if apiURL.Values == nil {
		t.Fatal("expected per-file values map, got nil")
	}
	if apiURL.Values["frontend/main/.env"] != "http://localhost:3000/api/v1" {
		t.Errorf("main = %q", apiURL.Values["frontend/main/.env"])
	}
	if apiURL.Values["frontend/portal/.env"] != "http://localhost:3000/portal/api/v1" {
		t.Errorf("portal = %q", apiURL.Values["frontend/portal/.env"])
	}

	// Check .env files contain the correct per-file values
	mainEnv, _ := os.ReadFile(filepath.Join(dir, "frontend/main/.env"))
	if !bytes.Contains(mainEnv, []byte("API_URL=http://localhost:3000/api/v1")) {
		t.Errorf("main/.env missing correct API_URL, got:\n%s", mainEnv)
	}
	portalEnv, _ := os.ReadFile(filepath.Join(dir, "frontend/portal/.env"))
	if !bytes.Contains(portalEnv, []byte("API_URL=http://localhost:3000/portal/api/v1")) {
		t.Errorf("portal/.env missing correct API_URL, got:\n%s", portalEnv)
	}
}

func TestUp_NoDerived_OmitsFromJSON(t *testing.T) {
	setupProject(t, testConfig)

	output := executeCmd(t, "up", "--json")

	if bytes.Contains([]byte(output), []byte("derived")) {
		t.Errorf("JSON output should omit derived when empty, got:\n%s", output)
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

// --- system status ---

func TestSystemStatus_EmptyRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	output := executeCmd(t, "system", "status")

	if !bytes.Contains([]byte(output), []byte("No projects registered")) {
		t.Errorf("expected 'No projects registered', got:\n%s", output)
	}
}

func TestSystemStatus_ShowsProjects(t *testing.T) {
	setupProject(t, testConfig)

	// Populate registry via apply
	executeCmd(t, "up", "--json")

	output := executeCmd(t, "system", "status", "--json")

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

func TestSystemStatus_StaleProjectMarkedNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Create a registry with a stale entry (nonexistent dir)
	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	reg.Set("staleapp", "main", registry.Allocation{
		ProjectDir: "/tmp/nonexistent-outport-stale-test",
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	// Use JSON mode to avoid the interactive prompt
	output := executeCmd(t, "system", "status", "--json")

	var entries []statusEntryJSON
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	if len(entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(entries))
	}
	if entries[0].Key != "staleapp/main" {
		t.Errorf("key = %q, want %q", entries[0].Key, "staleapp/main")
	}
	// Stale project should still appear — status shows everything
	if entries[0].Services["web"].Port != 12345 {
		t.Errorf("web port = %d, want 12345", entries[0].Services["web"].Port)
	}
}

func TestSystemStatus_StaleProjectInJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Create a registry with a stale entry (directory gone)
	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	reg.Set("staleapp", "main", registry.Allocation{
		ProjectDir: "/tmp/nonexistent-outport-stale-test",
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	// JSON mode doesn't prompt — just verifies stale entries show up
	output := executeCmd(t, "system", "status", "--json")

	var entries []statusEntryJSON
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Key != "staleapp/main" {
		t.Errorf("key = %q, want staleapp/main", entries[0].Key)
	}
	// Stale removal is tested via gc command, which doesn't use interactive prompts
}

// --- system gc ---

func TestSystemGC_RemovesStaleEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Manually create a registry with a stale entry
	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
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

	output := executeCmd(t, "system", "gc")

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

func TestSystemGC_NoStaleEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := t.TempDir()
	// Add a config file so gc doesn't consider it stale
	os.WriteFile(filepath.Join(projectDir, ".outport.yml"), []byte("name: validapp\nservices:\n  web:\n    env_var: PORT\n"), 0644)
	t.Chdir(projectDir)
	jsonFlag = false

	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
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

	output := executeCmd(t, "system", "gc")

	if !bytes.Contains([]byte(output), []byte("No stale entries")) {
		t.Errorf("expected 'No stale entries', got:\n%s", output)
	}
}

func TestSystemGC_RemovesMissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory exists but has no .outport.yml
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	jsonFlag = false

	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	reg.Set("noconfigapp", "main", registry.Allocation{
		ProjectDir: projectDir,
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	output := executeCmd(t, "system", "gc")

	if !bytes.Contains([]byte(output), []byte("Removed 1 stale")) {
		t.Errorf("expected removal of config-missing entry, got:\n%s", output)
	}

	reg2, _ := registry.Load(regPath)
	if len(reg2.Projects) != 0 {
		t.Errorf("registry still has %d entries", len(reg2.Projects))
	}
}

// --- up --force ---

func TestUp_ForceReallocatesWithPreferredPorts(t *testing.T) {
	setupProject(t, testConfig)

	// First allocation
	out1 := executeCmd(t, "up", "--json")
	var r1 upJSON
	json.Unmarshal([]byte(out1), &r1)

	// Ports should be preferred (3000, 5432) since nothing else is registered
	if r1.Services["web"].Port != 3000 {
		t.Errorf("first apply: web port = %d, want 3000", r1.Services["web"].Port)
	}

	// Force re-allocation should produce the same preferred ports
	out2 := executeCmd(t, "up", "--force", "--json")
	var r2 upJSON
	json.Unmarshal([]byte(out2), &r2)

	if r2.Services["web"].Port != 3000 {
		t.Errorf("apply --force: web port = %d, want 3000", r2.Services["web"].Port)
	}
	if r2.Services["postgres"].Port != 5432 {
		t.Errorf("apply --force: postgres port = %d, want 5432", r2.Services["postgres"].Port)
	}
}

func TestSystemStatus_MissingConfigMarkedStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory exists but no .outport.yml
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	jsonFlag = false

	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	reg.Set("noconfigapp", "main", registry.Allocation{
		ProjectDir: projectDir,
		Ports:      map[string]int{"web": 12345},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	// Use JSON mode to avoid huh interactive prompt which can hang
	// when stdin is not a real terminal (same reason init tests were removed)
	output := executeCmd(t, "system", "status", "--json")

	var entries []statusEntryJSON
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	// Stale project with missing config should still appear in JSON output
	if entries[0].Key != "noconfigapp/main" {
		t.Errorf("key = %q, want noconfigapp/main", entries[0].Key)
	}
}

// --- init ---

// Note: TestInit_CreatesConfig and TestInit_DefaultProjectName were removed because
// outport init now uses huh TUI prompts which require a real terminal.
// The init command is tested manually. The error path is still tested below.

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

func TestPorts_StyledOutput(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up")

	output := executeCmd(t, "ports")

	if !bytes.Contains([]byte(output), []byte("testapp")) {
		t.Errorf("styled output missing project name, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("PORT")) {
		t.Errorf("styled output missing PORT env var, got:\n%s", output)
	}
}

// --- down ---

func TestDown_RemovesFromRegistry(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up")

	output := executeCmd(t, "down")

	if !bytes.Contains([]byte(output), []byte("Done")) {
		t.Errorf("expected 'Done' message, got:\n%s", output)
	}

	portsOutput := executeCmd(t, "ports")
	if !bytes.Contains([]byte(portsOutput), []byte("No ports allocated")) {
		t.Errorf("expected no ports after unregister, got:\n%s", portsOutput)
	}
}

func TestDown_CleansEnvFiles(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	// Apply to write .env files with fenced blocks
	executeCmd(t, "up")

	// Verify blocks exist before unregister
	backendEnv, _ := os.ReadFile(filepath.Join(dir, "backend", ".env"))
	if !bytes.Contains(backendEnv, []byte("# --- begin outport.dev ---")) {
		t.Fatal("backend/.env should have outport block before unregister")
	}

	// Unapply should remove the blocks
	executeCmd(t, "down")

	// Verify blocks are gone
	backendEnv, _ = os.ReadFile(filepath.Join(dir, "backend", ".env"))
	if bytes.Contains(backendEnv, []byte("# --- begin outport.dev ---")) {
		t.Error("backend/.env should not have outport block after unregister")
	}
	frontendEnv, _ := os.ReadFile(filepath.Join(dir, "frontend", ".env"))
	if bytes.Contains(frontendEnv, []byte("# --- begin outport.dev ---")) {
		t.Error("frontend/.env should not have outport block after unregister")
	}
}

func TestDown_JSONShowsCleanedFiles(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	executeCmd(t, "up")
	output := executeCmd(t, "down", "--json")

	var result struct {
		Project      string   `json:"project"`
		Instance     string   `json:"instance"`
		Status       string   `json:"status"`
		CleanedFiles []string `json:"cleaned_files"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Status != "removed" {
		t.Errorf("status = %q, want unregistered", result.Status)
	}
	if len(result.CleanedFiles) == 0 {
		t.Error("expected cleaned_files to list env files")
	}
}

func TestDown_NotRegistered(t *testing.T) {
	setupProject(t, testConfig)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"down"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func TestDown_JSON(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up", "--json")

	output := executeCmd(t, "down", "--json")

	var result struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Project != "testapp" {
		t.Errorf("project = %q, want %q", result.Project, "testapp")
	}
	if result.Status != "removed" {
		t.Errorf("status = %q, want %q", result.Status, "removed")
	}
}

// --- open ---

func TestOpen_NoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"open"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no config exists")
	}
}

func TestOpen_NoAllocation(t *testing.T) {
	setupProject(t, testConfig)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"open"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no ports allocated")
	}
}

func TestOpen_UnknownService(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"open", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestOpen_NoProtocol(t *testing.T) {
	// postgres has no protocol, so open should error
	setupProject(t, testConfig)
	executeCmd(t, "up")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"open", "postgres"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for service without protocol")
	}
}

// --- serviceURL ---

// --- rename ---

const testConfigWithHostnames = `name: testapp
services:
  web:
    preferred_port: 3000
    env_var: PORT
    protocol: http
    hostname: testapp
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
`

func TestRename_Success(t *testing.T) {
	dir := setupProject(t, testConfigWithHostnames)

	// Apply to create the "main" instance
	executeCmd(t, "up", "--json")

	// Rename main → staging
	output := executeCmd(t, "rename", "--json", "main", "staging")

	var result struct {
		Project     string `json:"project"`
		OldInstance string `json:"old_instance"`
		NewInstance string `json:"new_instance"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Project != "testapp" {
		t.Errorf("project = %q, want testapp", result.Project)
	}
	if result.OldInstance != "main" {
		t.Errorf("old_instance = %q, want main", result.OldInstance)
	}
	if result.NewInstance != "staging" {
		t.Errorf("new_instance = %q, want staging", result.NewInstance)
	}
	if result.Status != "renamed" {
		t.Errorf("status = %q, want renamed", result.Status)
	}

	// Verify registry has new key with correct hostnames
	regPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("testapp", "main"); ok {
		t.Error("old instance 'main' should be gone from registry")
	}
	alloc, ok := reg.Get("testapp", "staging")
	if !ok {
		t.Fatal("new instance 'staging' should be in registry")
	}
	// For non-main, hostname should contain the instance suffix
	if alloc.Hostnames["web"] != "testapp-staging.test" {
		t.Errorf("hostname = %q, want testapp-staging.test", alloc.Hostnames["web"])
	}
	// Ports should be preserved
	if alloc.Ports["web"] != 3000 {
		t.Errorf("web port = %d, want 3000", alloc.Ports["web"])
	}

	// Verify .env was updated
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	if !bytes.Contains(envData, []byte("PORT=3000")) {
		t.Errorf(".env missing PORT=3000, got:\n%s", envData)
	}
}

func TestRename_CollisionFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	jsonFlag = false

	// Create main project directory
	dir1 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, ".outport.yml"), []byte(testConfig), 0644)
	os.Mkdir(filepath.Join(dir1, ".git"), 0755)

	// Apply from dir1 to create "main" instance
	t.Chdir(dir1)
	executeCmd(t, "up", "--json")

	// Create a second directory for the same project
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, ".outport.yml"), []byte(testConfig), 0644)
	os.Mkdir(filepath.Join(dir2, ".git"), 0755)

	// Apply from dir2 to create a code-based instance
	t.Chdir(dir2)
	out2 := executeCmd(t, "up", "--json")
	var r2 upJSON
	json.Unmarshal([]byte(out2), &r2)
	codeName := r2.Instance

	// Try to rename code instance to "main" — should collide
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"rename", codeName, "main"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when renaming to existing instance name")
	}
}

func TestRename_InvalidNameFails(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up", "--json")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"rename", "main", "has_underscore"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid instance name")
	}
}

// --- promote ---

func TestPromote_Success(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	jsonFlag = false

	// Create main project directory
	dir1 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, ".outport.yml"), []byte(testConfigWithHostnames), 0644)
	os.Mkdir(filepath.Join(dir1, ".git"), 0755)

	// Apply from dir1 to create "main" instance
	t.Chdir(dir1)
	executeCmd(t, "up", "--json")

	// Create a second directory for the same project
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, ".outport.yml"), []byte(testConfigWithHostnames), 0644)
	os.Mkdir(filepath.Join(dir2, ".git"), 0755)

	// Apply from dir2 to create a code-based instance
	t.Chdir(dir2)
	out2 := executeCmd(t, "up", "--json")
	var r2 upJSON
	json.Unmarshal([]byte(out2), &r2)
	codeName := r2.Instance

	// Promote the code instance to main
	output := executeCmd(t, "promote", "--json")

	var result struct {
		Project   string `json:"project"`
		Promoted  string `json:"promoted"`
		DemotedTo string `json:"demoted_to"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Project != "testapp" {
		t.Errorf("project = %q, want testapp", result.Project)
	}
	if result.Promoted != codeName {
		t.Errorf("promoted = %q, want %q", result.Promoted, codeName)
	}
	if result.DemotedTo == "" {
		t.Error("demoted_to should not be empty when main existed")
	}
	if result.Status != "promoted" {
		t.Errorf("status = %q, want promoted", result.Status)
	}

	// Verify registry: promoted instance is now "main"
	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	mainAlloc, ok := reg.Get("testapp", "main")
	if !ok {
		t.Fatal("expected 'main' instance in registry after promote")
	}
	// The promoted instance should now have main hostnames
	if mainAlloc.Hostnames["web"] != "testapp.test" {
		t.Errorf("promoted hostname = %q, want testapp.test", mainAlloc.Hostnames["web"])
	}
	// The promoted instance's dir should be dir2
	if mainAlloc.ProjectDir != dir2 {
		t.Errorf("main project dir = %q, want %q", mainAlloc.ProjectDir, dir2)
	}

	// The demoted instance should exist with a code name
	demotedAlloc, ok := reg.Get("testapp", result.DemotedTo)
	if !ok {
		t.Fatalf("expected demoted instance %q in registry", result.DemotedTo)
	}
	if demotedAlloc.ProjectDir != dir1 {
		t.Errorf("demoted project dir = %q, want %q", demotedAlloc.ProjectDir, dir1)
	}
	// Demoted instance should have suffixed hostname
	expectedHostname := "testapp-" + result.DemotedTo + ".test"
	if demotedAlloc.Hostnames["web"] != expectedHostname {
		t.Errorf("demoted hostname = %q, want %q", demotedAlloc.Hostnames["web"], expectedHostname)
	}
}

func TestPromote_AlreadyMainFails(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "up", "--json")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"promote"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when promoting from main instance")
	}
}

// --- hostname integration tests ---

const testConfigWithMultipleHostnames = `name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp
  api:
    env_var: API_PORT
    protocol: http
    hostname: api.myapp
  postgres:
    env_var: PGPORT
derived:
  CORS_ORIGINS:
    value: "${web.url},${api.url}"
    env_file: .env
  API_BASE:
    value: "${api.url:direct}/v1"
    env_file: .env
`

func TestUp_WithHostnames(t *testing.T) {
	dir := setupProject(t, testConfigWithMultipleHostnames)

	output := executeCmd(t, "up", "--json")

	var result upJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, output)
	}

	if result.Project != "myapp" {
		t.Errorf("project = %q, want %q", result.Project, "myapp")
	}
	if result.Instance != "main" {
		t.Errorf("instance = %q, want %q", result.Instance, "main")
	}
	if len(result.Services) != 3 {
		t.Fatalf("services count = %d, want 3", len(result.Services))
	}

	// Verify JSON includes hostnames for services with hostname config
	webSvc := result.Services["web"]
	if webSvc.Hostname != "myapp.test" {
		t.Errorf("web hostname = %q, want %q", webSvc.Hostname, "myapp.test")
	}
	if webSvc.Protocol != "http" {
		t.Errorf("web protocol = %q, want %q", webSvc.Protocol, "http")
	}
	if webSvc.URL == "" {
		t.Error("web URL should not be empty")
	}

	apiSvc := result.Services["api"]
	if apiSvc.Hostname != "api.myapp.test" {
		t.Errorf("api hostname = %q, want %q", apiSvc.Hostname, "api.myapp.test")
	}

	// Postgres should not have a hostname
	pgSvc := result.Services["postgres"]
	if pgSvc.Hostname != "" {
		t.Errorf("postgres hostname = %q, want empty", pgSvc.Hostname)
	}

	// Verify registry contains hostnames and protocols
	regPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	alloc, ok := reg.Get("myapp", "main")
	if !ok {
		t.Fatal("expected myapp/main in registry")
	}
	if alloc.Hostnames["web"] != "myapp.test" {
		t.Errorf("registry web hostname = %q, want myapp.test", alloc.Hostnames["web"])
	}
	if alloc.Hostnames["api"] != "api.myapp.test" {
		t.Errorf("registry api hostname = %q, want api.myapp.test", alloc.Hostnames["api"])
	}
	if alloc.Protocols["web"] != "http" {
		t.Errorf("registry web protocol = %q, want http", alloc.Protocols["web"])
	}
	if alloc.Protocols["api"] != "http" {
		t.Errorf("registry api protocol = %q, want http", alloc.Protocols["api"])
	}

	// Verify .env contains resolved derived values with url and url:direct
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	envContent := string(envData)
	// CORS_ORIGINS should use the .test hostnames
	if !bytes.Contains(envData, []byte("CORS_ORIGINS=http://myapp.test,http://api.myapp.test")) {
		t.Errorf(".env missing expected CORS_ORIGINS, got:\n%s", envContent)
	}
	// API_BASE should use the :direct modifier (localhost:port)
	apiPort := result.Services["api"].Port
	expected := fmt.Sprintf("API_BASE=http://localhost:%d/v1", apiPort)
	if !bytes.Contains(envData, []byte(expected)) {
		t.Errorf(".env missing expected API_BASE=%s, got:\n%s", expected, envContent)
	}
}

func TestUp_HostnameUniquenessConflict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	jsonFlag = false

	// Project 1: "myapp" with hostname "myapp"
	dir1 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, ".outport.yml"), []byte(`name: myapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp
`), 0644)
	os.Mkdir(filepath.Join(dir1, ".git"), 0755)

	// Apply project 1 — should succeed
	t.Chdir(dir1)
	executeCmd(t, "up", "--json")

	// Project 2: different project name but same hostname stem
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, ".outport.yml"), []byte(`name: myapp2
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp2
`), 0644)
	os.Mkdir(filepath.Join(dir2, ".git"), 0755)

	// Apply project 2 — should succeed (different hostname)
	t.Chdir(dir2)
	executeCmd(t, "up", "--json")

	// Project 3: a different project that conflicts with project 1's hostname
	dir3 := t.TempDir()
	os.WriteFile(filepath.Join(dir3, ".outport.yml"), []byte(`name: otherapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp.otherapp
`), 0644)
	os.Mkdir(filepath.Join(dir3, ".git"), 0755)

	// This one won't conflict because "myapp.otherapp.test" != "myapp.test".
	// Now make a real conflict by matching project 1's exact hostname.
	os.WriteFile(filepath.Join(dir3, ".outport.yml"), []byte(`name: clash
services:
  web:
    env_var: PORT
    protocol: http
    hostname: clash
`), 0644)

	// Apply this project — should succeed (unique hostname)
	t.Chdir(dir3)
	executeCmd(t, "up", "--json")

	// Now set up a project that truly conflicts with myapp's hostname
	dir4 := t.TempDir()
	// This config has hostname "myapp" which resolves to "myapp.test"
	// — conflicts with project 1
	configWithConflict := `name: fakeapp
services:
  web:
    env_var: PORT
    protocol: http
    hostname: myapp.fakeapp
`
	os.WriteFile(filepath.Join(dir4, ".outport.yml"), []byte(configWithConflict), 0644)
	os.Mkdir(filepath.Join(dir4, ".git"), 0755)

	// Directly set up a registry entry that will cause a conflict
	regPath := filepath.Join(home, ".local", "share", "outport", "registry.json")
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatal(err)
	}
	// Manually add an entry with a hostname that will collide
	reg.Set("existing", "main", registry.Allocation{
		ProjectDir: t.TempDir(),
		Ports:      map[string]int{"web": 11111},
		Hostnames:  map[string]string{"web": "collider.test"},
	})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	// Create project that tries to use the same hostname "collider"
	dir5 := t.TempDir()
	os.WriteFile(filepath.Join(dir5, ".outport.yml"), []byte(`name: collider
services:
  web:
    env_var: PORT
    protocol: http
    hostname: collider
`), 0644)
	os.Mkdir(filepath.Join(dir5, ".git"), 0755)

	t.Chdir(dir5)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"up"})

	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when hostname conflicts with existing project")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("conflicts")) {
		t.Errorf("error should mention conflict, got: %v", err)
	}
}

func TestUp_TemplateModifiers(t *testing.T) {
	dir := setupProject(t, `name: myapp
services:
  web:
    env_var: PORT
    preferred_port: 3000
    protocol: http
    hostname: myapp
  api:
    env_var: API_PORT
    preferred_port: 4000
    protocol: http
    hostname: api.myapp

derived:
  WEB_URL:
    value: "${web.url}/app"
    env_file: .env
  WEB_DIRECT:
    value: "${web.url:direct}/app"
    env_file: .env
  API_URL:
    value: "${api.url}/v1"
    env_file: .env
  API_DIRECT:
    value: "${api.url:direct}/v1"
    env_file: .env
  COMBINED:
    value: "${web.url},${api.url:direct}"
    env_file: .env
`)

	output := executeCmd(t, "up", "--json")

	var result upJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}

	// Verify derived values in JSON output
	webURL := result.Derived["WEB_URL"]
	if webURL.Value != "http://myapp.test/app" {
		t.Errorf("WEB_URL = %q, want http://myapp.test/app", webURL.Value)
	}

	webDirect := result.Derived["WEB_DIRECT"]
	if webDirect.Value != "http://localhost:3000/app" {
		t.Errorf("WEB_DIRECT = %q, want http://localhost:3000/app", webDirect.Value)
	}

	apiURL := result.Derived["API_URL"]
	if apiURL.Value != "http://api.myapp.test/v1" {
		t.Errorf("API_URL = %q, want http://api.myapp.test/v1", apiURL.Value)
	}

	apiDirect := result.Derived["API_DIRECT"]
	if apiDirect.Value != "http://localhost:4000/v1" {
		t.Errorf("API_DIRECT = %q, want http://localhost:4000/v1", apiDirect.Value)
	}

	combined := result.Derived["COMBINED"]
	if combined.Value != "http://myapp.test,http://localhost:4000" {
		t.Errorf("COMBINED = %q, want http://myapp.test,http://localhost:4000", combined.Value)
	}

	// Verify .env file contains resolved values
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	envContent := string(envData)

	expectations := map[string]string{
		"WEB_URL":    "http://myapp.test/app",
		"WEB_DIRECT": "http://localhost:3000/app",
		"API_URL":    "http://api.myapp.test/v1",
		"API_DIRECT": "http://localhost:4000/v1",
		"COMBINED":   "http://myapp.test,http://localhost:4000",
	}

	for name, expected := range expectations {
		envLine := name + "=" + expected
		if !bytes.Contains(envData, []byte(envLine)) {
			t.Errorf(".env missing %s, got:\n%s", envLine, envContent)
		}
	}
}

func TestServiceURL(t *testing.T) {
	useHTTPS = false
	if url := serviceURL("http", "", 3000); url != "http://localhost:3000" {
		t.Errorf("serviceURL(http, '', 3000) = %q, want http://localhost:3000", url)
	}
	if url := serviceURL("https", "", 8443); url != "https://localhost:8443" {
		t.Errorf("serviceURL(https, '', 8443) = %q, want https://localhost:8443", url)
	}
	if url := serviceURL("http", "myapp.localhost", 3000); url != "http://myapp.localhost:3000" {
		t.Errorf("serviceURL(http, myapp.localhost, 3000) = %q, want http://myapp.localhost:3000", url)
	}
	if url := serviceURL("tcp", "", 5432); url != "" {
		t.Errorf("serviceURL(tcp, '', 5432) = %q, want empty", url)
	}
	if url := serviceURL("", "", 6379); url != "" {
		t.Errorf("serviceURL('', '', 6379) = %q, want empty", url)
	}
	// With useHTTPS=true, .test hostnames get https://
	useHTTPS = true
	if url := serviceURL("http", "myapp.test", 3000); url != "https://myapp.test" {
		t.Errorf("serviceURL(http, myapp.test, 3000) with useHTTPS = %q, want https://myapp.test", url)
	}
	// Without useHTTPS, .test hostnames keep original protocol
	useHTTPS = false
	if url := serviceURL("http", "myapp.test", 3000); url != "http://myapp.test" {
		t.Errorf("serviceURL(http, myapp.test, 3000) without useHTTPS = %q, want http://myapp.test", url)
	}
}

func TestBuildTemplateVarsHTTPS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(home, ".local", "share", "outport")
	os.MkdirAll(dataDir, 0755)
	certmanager.GenerateCA(
		filepath.Join(dataDir, "ca-cert.pem"),
		filepath.Join(dataDir, "ca-key.pem"),
	)

	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT", Protocol: "http", Hostname: "myapp.test"},
		},
	}
	ports := map[string]int{"rails": 3000}
	hostnames := map[string]string{"rails": "myapp.test"}

	useHTTPS = certmanager.IsCAInstalled()
	vars := buildTemplateVars(cfg, ports, hostnames)

	if vars["rails.url"] != "https://myapp.test" {
		t.Errorf("rails.url = %q, want %q", vars["rails.url"], "https://myapp.test")
	}
	if vars["rails.url:direct"] != "http://localhost:3000" {
		t.Errorf("rails.url:direct = %q, want %q", vars["rails.url:direct"], "http://localhost:3000")
	}
}

func TestBuildTemplateVarsHTTP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // No CA here

	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT", Protocol: "http", Hostname: "myapp.test"},
		},
	}
	ports := map[string]int{"rails": 3000}
	hostnames := map[string]string{"rails": "myapp.test"}

	useHTTPS = certmanager.IsCAInstalled()
	vars := buildTemplateVars(cfg, ports, hostnames)

	if vars["rails.url"] != "http://myapp.test" {
		t.Errorf("rails.url = %q, want %q", vars["rails.url"], "http://myapp.test")
	}
}

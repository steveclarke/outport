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

// --- apply ---

func TestApply_AllocatesPortsAndWritesEnv(t *testing.T) {
	dir := setupProject(t, testConfig)

	output := executeCmd(t, "apply", "--json")

	var result applyJSON
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

func TestApply_IsIdempotent(t *testing.T) {
	setupProject(t, testConfig)

	out1 := executeCmd(t, "apply", "--json")
	out2 := executeCmd(t, "apply", "--json")

	var r1, r2 applyJSON
	json.Unmarshal([]byte(out1), &r1)
	json.Unmarshal([]byte(out2), &r2)

	if r1.Services["web"].Port != r2.Services["web"].Port {
		t.Errorf("web port changed: %d -> %d", r1.Services["web"].Port, r2.Services["web"].Port)
	}
	if r1.Services["postgres"].Port != r2.Services["postgres"].Port {
		t.Errorf("postgres port changed: %d -> %d", r1.Services["postgres"].Port, r2.Services["postgres"].Port)
	}
}

func TestApply_StyledOutput(t *testing.T) {
	setupProject(t, testConfig)

	output := executeCmd(t, "apply")

	if !bytes.Contains([]byte(output), []byte("testapp")) {
		t.Errorf("styled output missing project name, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Ports written to .env")) {
		t.Errorf("styled output missing success message, got:\n%s", output)
	}
}

func TestApply_NoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	t.Chdir(dir)
	jsonFlag = false

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"apply"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no .outport.yml exists")
	}
}

// --- apply with derived values ---

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

func TestApply_WithDerivedValues(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	output := executeCmd(t, "apply", "--json")

	var result applyJSON
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

func TestApply_DerivedStyledOutput(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	output := executeCmd(t, "apply")

	if !bytes.Contains([]byte(output), []byte("derived:")) {
		t.Errorf("styled output missing 'derived:' section, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("API_URL")) {
		t.Errorf("styled output missing API_URL, got:\n%s", output)
	}
}

func TestApply_DerivedPerFileValues(t *testing.T) {
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

	output := executeCmd(t, "apply", "--json")

	var result applyJSON
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

func TestApply_NoDerived_OmitsFromJSON(t *testing.T) {
	setupProject(t, testConfig)

	output := executeCmd(t, "apply", "--json")

	if bytes.Contains([]byte(output), []byte("derived")) {
		t.Errorf("JSON output should omit derived when empty, got:\n%s", output)
	}
}

// --- ports ---

func TestPorts_ShowsAllocatedPorts(t *testing.T) {
	setupProject(t, testConfig)

	// First allocate ports
	executeCmd(t, "apply", "--json")

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

	// Populate registry via apply
	executeCmd(t, "apply", "--json")

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

func TestStatus_StaleProjectMarkedNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Create a registry with a stale entry (nonexistent dir)
	regPath := filepath.Join(home, ".config", "outport", "registry.json")
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
	output := executeCmd(t, "status", "--json")

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

func TestStatus_StaleProjectInJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	jsonFlag = false

	// Create a registry with a stale entry (directory gone)
	regPath := filepath.Join(home, ".config", "outport", "registry.json")
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
	output := executeCmd(t, "status", "--json")

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
	// Add a config file so gc doesn't consider it stale
	os.WriteFile(filepath.Join(projectDir, ".outport.yml"), []byte("name: validapp\nservices:\n  web:\n    env_var: PORT\n"), 0644)
	t.Chdir(projectDir)
	jsonFlag = false

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

func TestGC_RemovesMissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory exists but has no .outport.yml
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	jsonFlag = false

	regPath := filepath.Join(home, ".config", "outport", "registry.json")
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

	output := executeCmd(t, "gc")

	if !bytes.Contains([]byte(output), []byte("Removed 1 stale")) {
		t.Errorf("expected removal of config-missing entry, got:\n%s", output)
	}

	reg2, _ := registry.Load(regPath)
	if len(reg2.Projects) != 0 {
		t.Errorf("registry still has %d entries", len(reg2.Projects))
	}
}

// --- apply --force ---

func TestApply_ForceReallocatesWithPreferredPorts(t *testing.T) {
	setupProject(t, testConfig)

	// First allocation
	out1 := executeCmd(t, "apply", "--json")
	var r1 applyJSON
	json.Unmarshal([]byte(out1), &r1)

	// Ports should be preferred (3000, 5432) since nothing else is registered
	if r1.Services["web"].Port != 3000 {
		t.Errorf("first apply: web port = %d, want 3000", r1.Services["web"].Port)
	}

	// Force re-allocation should produce the same preferred ports
	out2 := executeCmd(t, "apply", "--force", "--json")
	var r2 applyJSON
	json.Unmarshal([]byte(out2), &r2)

	if r2.Services["web"].Port != 3000 {
		t.Errorf("apply --force: web port = %d, want 3000", r2.Services["web"].Port)
	}
	if r2.Services["postgres"].Port != 5432 {
		t.Errorf("apply --force: postgres port = %d, want 5432", r2.Services["postgres"].Port)
	}
}

func TestStatus_MissingConfigMarkedStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory exists but no .outport.yml
	projectDir := t.TempDir()
	t.Chdir(projectDir)
	jsonFlag = false

	regPath := filepath.Join(home, ".config", "outport", "registry.json")
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
	output := executeCmd(t, "status", "--json")

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
	executeCmd(t, "apply")

	output := executeCmd(t, "ports")

	if !bytes.Contains([]byte(output), []byte("testapp")) {
		t.Errorf("styled output missing project name, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("PORT")) {
		t.Errorf("styled output missing PORT env var, got:\n%s", output)
	}
}

// --- unregister ---

func TestUnapply_RemovesFromRegistry(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "apply")

	output := executeCmd(t, "unapply")

	if !bytes.Contains([]byte(output), []byte("Unapplied")) {
		t.Errorf("expected 'Unapplied' message, got:\n%s", output)
	}

	portsOutput := executeCmd(t, "ports")
	if !bytes.Contains([]byte(portsOutput), []byte("No ports allocated")) {
		t.Errorf("expected no ports after unregister, got:\n%s", portsOutput)
	}
}

func TestUnapply_CleansEnvFiles(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	// Apply to write .env files with fenced blocks
	executeCmd(t, "apply")

	// Verify blocks exist before unregister
	backendEnv, _ := os.ReadFile(filepath.Join(dir, "backend", ".env"))
	if !bytes.Contains(backendEnv, []byte("# --- begin outport.dev ---")) {
		t.Fatal("backend/.env should have outport block before unregister")
	}

	// Unapply should remove the blocks
	executeCmd(t, "unapply")

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

func TestUnapply_JSONShowsCleanedFiles(t *testing.T) {
	dir := setupProject(t, testConfigWithDerived)
	os.MkdirAll(filepath.Join(dir, "backend"), 0755)
	os.MkdirAll(filepath.Join(dir, "frontend"), 0755)

	executeCmd(t, "apply")
	output := executeCmd(t, "unapply", "--json")

	var result struct {
		Project      string   `json:"project"`
		Instance     string   `json:"instance"`
		Status       string   `json:"status"`
		CleanedFiles []string `json:"cleaned_files"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, output)
	}
	if result.Status != "unapplied" {
		t.Errorf("status = %q, want unregistered", result.Status)
	}
	if len(result.CleanedFiles) == 0 {
		t.Error("expected cleaned_files to list env files")
	}
}

func TestUnapply_NotRegistered(t *testing.T) {
	setupProject(t, testConfig)

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"unapply"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when not registered")
	}
}

func TestUnapply_JSON(t *testing.T) {
	setupProject(t, testConfig)
	executeCmd(t, "apply", "--json")

	output := executeCmd(t, "unapply", "--json")

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
	if result.Status != "unapplied" {
		t.Errorf("status = %q, want %q", result.Status, "unapplied")
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
	executeCmd(t, "apply")

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
	executeCmd(t, "apply")

	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"open", "postgres"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for service without protocol")
	}
}

// --- serviceURL ---

func TestServiceURL(t *testing.T) {
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
}

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/outport-app/outport/internal/config"
)

func TestProjectChecksValidConfig(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")
	_ = os.WriteFile(regPath, []byte(`{"projects":{}}`), 0644)

	cfgYAML := `name: myapp
services:
  web:
    env_var: PORT
`
	_ = os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(cfgYAML), 0644)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}

	checks := ProjectChecks(dir, cfg, nil, regPath)
	if len(checks) < 2 {
		t.Fatalf("expected at least 2 checks (config + registered), got %d", len(checks))
	}

	// Config check should pass
	res := checks[0].Run()
	if res.Status != Pass {
		t.Errorf("expected config check Pass, got %v: %s", res.Status, res.Message)
	}

	// Registration check should fail (project not in registry)
	res = checks[1].Run()
	if res.Status != Fail {
		t.Errorf("expected registration Fail for unregistered project, got %v", res.Status)
	}
}

func TestProjectChecksInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")
	_ = os.WriteFile(regPath, []byte(`{"projects":{}}`), 0644)

	configErr := config.Load  // trigger an error by passing nil cfg
	_ = configErr

	// Simulate a config error
	checks := ProjectChecks(dir, nil, os.ErrNotExist, regPath)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check (config only), got %d", len(checks))
	}
	res := checks[0].Run()
	if res.Status != Fail {
		t.Errorf("expected config check Fail, got %v", res.Status)
	}
}

func TestProjectChecksRegistered(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Register the project
	_ = os.WriteFile(regPath, []byte(`{"projects":{"myapp/main":{"project_dir":"`+dir+`","ports":{"web":3000}}}}`), 0644)

	cfgYAML := `name: myapp
services:
  web:
    env_var: PORT
`
	_ = os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(cfgYAML), 0644)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}

	checks := ProjectChecks(dir, cfg, nil, regPath)
	// Should have: config valid + project registered + port check for web
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}

	// Registration should pass
	res := checks[1].Run()
	if res.Status != Pass {
		t.Errorf("expected registration Pass, got %v: %s", res.Status, res.Message)
	}
}

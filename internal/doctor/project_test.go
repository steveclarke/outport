package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConfigValid(t *testing.T) {
	dir := t.TempDir()

	// Valid config
	cfg := `name: myapp
services:
  web:
    env_var: PORT
`
	_ = os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte(cfg), 0644)
	res := checkConfigValid(dir)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}

	// Invalid config (missing name)
	_ = os.WriteFile(filepath.Join(dir, ".outport.yml"), []byte("services:\n  web:\n    env_var: PORT\n"), 0644)
	res = checkConfigValid(dir)
	if res.Status != Fail {
		t.Errorf("expected Fail, got %v", res.Status)
	}
}

func TestCheckProjectRegistered(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Empty registry
	_ = os.WriteFile(regPath, []byte(`{"projects":{}}`), 0644)
	res := checkProjectRegistered(regPath, dir)
	if res.Status != Fail {
		t.Errorf("expected Fail for unregistered project, got %v", res.Status)
	}

	// Registered
	_ = os.WriteFile(regPath, []byte(`{"projects":{"myapp/main":{"project_dir":"`+dir+`","ports":{"web":3000}}}}`), 0644)
	res = checkProjectRegistered(regPath, dir)
	if res.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", res.Status, res.Message)
	}
}

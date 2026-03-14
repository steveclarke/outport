package dotenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMerge_NewFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	ports := map[string]string{
		"PORT":          "31653",
		"DATABASE_PORT": "17842",
	}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653")
	}
	if !strings.Contains(content, "DATABASE_PORT=17842") {
		t.Error("missing DATABASE_PORT=17842")
	}
	if !strings.Contains(content, "# managed by outport") {
		t.Error("missing outport marker comment")
	}
}

func TestMerge_PreservesExistingVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET_KEY=abc123\nRAILS_ENV=development\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
	if !strings.Contains(content, "RAILS_ENV=development") {
		t.Error("lost existing RAILS_ENV")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing PORT=31653")
	}
}

func TestMerge_UpdatesExistingOutportVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "SECRET_KEY=abc123\nPORT=99999 # managed by outport\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if strings.Contains(content, "99999") {
		t.Error("old port value should be replaced")
	}
	if !strings.Contains(content, "PORT=31653") {
		t.Error("missing updated PORT=31653")
	}
	if !strings.Contains(content, "SECRET_KEY=abc123") {
		t.Error("lost existing SECRET_KEY")
	}
}

func TestMerge_DoesNotClobberUserVar(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	existing := "PORT=4000\n"
	os.WriteFile(envPath, []byte(existing), 0644)

	ports := map[string]string{"PORT": "31653"}
	if err := Merge(envPath, ports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(envPath)
	content := string(data)

	if !strings.Contains(content, "PORT=4000") {
		t.Error("user's PORT was clobbered")
	}
}

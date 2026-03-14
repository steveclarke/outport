package registry

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRegistry_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg == nil {
		t.Fatal("registry is nil")
	}
	if len(reg.Projects) != 0 {
		t.Errorf("new registry should have 0 projects, got %d", len(reg.Projects))
	}
}

func TestRegistry_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, _ := Load(path)

	alloc := Allocation{
		ProjectDir: "/Users/steve/src/myapp",
		Ports: map[string]int{
			"web":      31653,
			"postgres": 17842,
		},
	}
	reg.Set("myapp", "main", alloc)

	got, ok := reg.Get("myapp", "main")
	if !ok {
		t.Fatal("expected to find allocation")
	}
	if got.Ports["web"] != 31653 {
		t.Errorf("web port = %d, want 31653", got.Ports["web"])
	}
}

func TestRegistry_SaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, _ := Load(path)
	reg.Set("myapp", "main", Allocation{
		ProjectDir: "/Users/steve/src/myapp",
		Ports:      map[string]int{"web": 31653},
	})
	if err := reg.Save(); err != nil {
		t.Fatalf("save error: %v", err)
	}

	reg2, err := Load(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	got, ok := reg2.Get("myapp", "main")
	if !ok {
		t.Fatal("allocation lost after reload")
	}
	if got.Ports["web"] != 31653 {
		t.Errorf("web port = %d after reload, want 31653", got.Ports["web"])
	}
}

func TestRegistry_UsedPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, _ := Load(path)
	reg.Set("app1", "main", Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": 10001, "db": 10002},
	})
	reg.Set("app2", "main", Allocation{
		ProjectDir: "/src/app2",
		Ports:      map[string]int{"web": 10003},
	})

	used := reg.UsedPorts()
	if !used[10001] || !used[10002] || !used[10003] {
		t.Errorf("UsedPorts missing expected ports: %v", used)
	}
	if len(used) != 3 {
		t.Errorf("UsedPorts count = %d, want 3", len(used))
	}
}

func TestRegistry_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, _ := Load(path)
	reg.Set("myapp", "main", Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 10001},
	})

	reg.Remove("myapp", "main")
	_, ok := reg.Get("myapp", "main")
	if ok {
		t.Error("allocation should be removed")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, ".config/outport/registry.json") {
		t.Errorf("path = %q, want suffix .config/outport/registry.json", path)
	}
}

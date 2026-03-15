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

func TestAllocationWithHostnames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg := &Registry{Projects: make(map[string]Allocation), path: path}
	reg.Set("myapp", "main", Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"rails": 24920},
		Hostnames:  map[string]string{"rails": "myapp.test"},
		Protocols:  map[string]string{"rails": "http"},
	})

	err := reg.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	alloc, ok := loaded.Get("myapp", "main")
	if !ok {
		t.Fatal("expected allocation")
	}
	if alloc.Hostnames["rails"] != "myapp.test" {
		t.Errorf("hostname: got %q, want %q", alloc.Hostnames["rails"], "myapp.test")
	}
	if alloc.Protocols["rails"] != "http" {
		t.Errorf("protocol: got %q, want %q", alloc.Protocols["rails"], "http")
	}
}

func TestFindByDir(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("myapp", "main", Allocation{ProjectDir: "/src/myapp"})
	reg.Set("myapp", "bkrm", Allocation{ProjectDir: "/tmp/myapp-clone"})

	key, alloc, ok := reg.FindByDir("/src/myapp")
	if !ok {
		t.Fatal("expected to find by dir")
	}
	if key != "myapp/main" {
		t.Errorf("key: got %q, want %q", key, "myapp/main")
	}
	if alloc.ProjectDir != "/src/myapp" {
		t.Errorf("dir: got %q", alloc.ProjectDir)
	}

	_, _, ok = reg.FindByDir("/nonexistent")
	if ok {
		t.Error("expected not found for nonexistent dir")
	}
}

func TestFindByProject(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("myapp", "main", Allocation{ProjectDir: "/src/myapp"})
	reg.Set("myapp", "bkrm", Allocation{ProjectDir: "/tmp/myapp-clone"})
	reg.Set("other", "main", Allocation{ProjectDir: "/src/other"})

	instances := reg.FindByProject("myapp")
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	instances = reg.FindByProject("nonexistent")
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
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

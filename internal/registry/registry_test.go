package registry

import (
	"os"
	"path/filepath"
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

func TestAll(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("app1", "main", Allocation{ProjectDir: "/src/app1", Ports: map[string]int{"web": 10001}})
	reg.Set("app2", "main", Allocation{ProjectDir: "/src/app2", Ports: map[string]int{"web": 10002}})

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d entries, want 2", len(all))
	}

	// Verify it's a copy — mutating the returned map shouldn't affect the registry
	delete(all, "app1/main")
	if _, ok := reg.Get("app1", "main"); !ok {
		t.Error("deleting from All() result should not affect the registry")
	}
}

func TestFindHostname(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("app1", "main", Allocation{
		ProjectDir: "/src/app1",
		Hostnames:  map[string]string{"rails": "app1.test"},
	})
	reg.Set("app2", "main", Allocation{
		ProjectDir: "/src/app2",
		Hostnames:  map[string]string{"rails": "app2.test"},
	})

	// Find existing hostname belonging to another project
	key, found := reg.FindHostname("app1.test", "app2/main")
	if !found {
		t.Fatal("expected to find hostname conflict")
	}
	if key != "app1/main" {
		t.Errorf("conflicting key = %q, want %q", key, "app1/main")
	}

	// Self is excluded
	_, found = reg.FindHostname("app1.test", "app1/main")
	if found {
		t.Error("self should be excluded from hostname search")
	}

	// Non-existent hostname
	_, found = reg.FindHostname("nonexistent.test", "app1/main")
	if found {
		t.Error("should not find nonexistent hostname")
	}
}

func TestRemoveStale(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("app1", "main", Allocation{ProjectDir: "/src/app1"})
	reg.Set("app2", "main", Allocation{ProjectDir: "/src/app2"})
	reg.Set("app3", "main", Allocation{ProjectDir: "/src/app3"})

	removed := reg.RemoveStale(func(dir string) bool {
		return dir == "/src/app1" || dir == "/src/app3"
	})

	if len(removed) != 2 {
		t.Fatalf("removed %d entries, want 2", len(removed))
	}

	// app2 should still exist
	if _, ok := reg.Get("app2", "main"); !ok {
		t.Error("app2 should not have been removed")
	}

	// app1 and app3 should be gone
	if _, ok := reg.Get("app1", "main"); ok {
		t.Error("app1 should have been removed")
	}
	if _, ok := reg.Get("app3", "main"); ok {
		t.Error("app3 should have been removed")
	}
}

func TestRemoveStale_NoneStale(t *testing.T) {
	reg := &Registry{Projects: make(map[string]Allocation)}
	reg.Set("app1", "main", Allocation{ProjectDir: "/src/app1"})

	removed := reg.RemoveStale(func(dir string) bool { return false })
	if len(removed) != 0 {
		t.Errorf("removed %d entries, want 0", len(removed))
	}
}

func TestDefaultPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	expected := filepath.Join(home, ".local", "share", "outport", "registry.json")
	if path != expected {
		t.Errorf("DefaultPath() = %q, want %q", path, expected)
	}
}

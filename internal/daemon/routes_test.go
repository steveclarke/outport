package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/tunnel"
)

func TestBuildRoutes(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"rails": 24920, "postgres": 5432},
		Hostnames:  map[string]string{"rails": "myapp.test"},
	})

	routes := BuildRoutes(reg)
	if routes["myapp.test"].Port != 24920 {
		t.Errorf("myapp.test: got %d, want 24920", routes["myapp.test"].Port)
	}
	if _, ok := routes["postgres"]; ok {
		t.Error("postgres should not have a route")
	}
}

func TestBuildRoutesSkipsNilHostnames(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("old", "main", registry.Allocation{
		ProjectDir: "/src/old",
		Ports:      map[string]int{"web": 12000},
	})

	routes := BuildRoutes(reg)
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestBuildRoutesIncludesAllHostnames(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 24920},
		Hostnames:  map[string]string{"web": "myapp.test"},
	})

	routes := BuildRoutes(reg)
	if routes["myapp.test"].Port != 24920 {
		t.Errorf("myapp.test: got %d, want 24920", routes["myapp.test"].Port)
	}
}

func TestBuildRoutesMultipleProjects(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "app1.test"},
	})
	reg.Set("app2", "main", registry.Allocation{
		ProjectDir: "/src/app2",
		Ports:      map[string]int{"web": 10002},
		Hostnames:  map[string]string{"web": "app2.test"},
	})

	routes := BuildRoutes(reg)
	if routes["app1.test"].Port != 10001 {
		t.Errorf("app1.test: got %d, want 10001", routes["app1.test"].Port)
	}
	if routes["app2.test"].Port != 10002 {
		t.Errorf("app2.test: got %d, want 10002", routes["app2.test"].Port)
	}
}

func TestBuildRoutesIncludesAliases(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("approvethis", "main", registry.Allocation{
		ProjectDir: "/src/approvethis",
		Ports:      map[string]int{"web": 14139},
		Hostnames:  map[string]string{"web": "approvethis.test"},
		Aliases: map[string]map[string]string{
			"web": {"app": "app.approvethis.test", "admin": "admin.approvethis.test"},
		},
	})

	routes := BuildRoutes(reg)

	if routes["approvethis.test"].Port != 14139 {
		t.Errorf("primary: got %d, want 14139", routes["approvethis.test"].Port)
	}
	if routes["app.approvethis.test"].Port != 14139 {
		t.Errorf("alias app: got %d, want 14139", routes["app.approvethis.test"].Port)
	}
	if routes["admin.approvethis.test"].Port != 14139 {
		t.Errorf("alias admin: got %d, want 14139", routes["admin.approvethis.test"].Port)
	}
	if routes["approvethis.test"].HostOverride != "" {
		t.Errorf("primary should have empty HostOverride")
	}
}

func TestBuildTunnelRoutes(t *testing.T) {
	state := &tunnel.TunnelState{
		HostnameMap: map[string]string{
			"abc123.trycloudflare.com": "approvethis.test",
			"def456.trycloudflare.com": "app.approvethis.test",
		},
	}
	allocs := map[string]registry.Allocation{
		"approvethis/main": {
			Ports:     map[string]int{"web": 14139},
			Hostnames: map[string]string{"web": "approvethis.test"},
			Aliases:   map[string]map[string]string{"web": {"app": "app.approvethis.test"}},
		},
	}

	routes := BuildTunnelRoutes(state, allocs)

	if len(routes) != 2 {
		t.Fatalf("expected 2 tunnel routes, got %d", len(routes))
	}
	r := routes["abc123.trycloudflare.com"]
	if r.Port != 14139 || r.HostOverride != "approvethis.test" {
		t.Errorf("primary tunnel route: got port=%d override=%q", r.Port, r.HostOverride)
	}
	r = routes["def456.trycloudflare.com"]
	if r.Port != 14139 || r.HostOverride != "app.approvethis.test" {
		t.Errorf("alias tunnel route: got port=%d override=%q", r.Port, r.HostOverride)
	}
}

func TestBuildTunnelRoutes_NilState(t *testing.T) {
	routes := BuildTunnelRoutes(nil, map[string]registry.Allocation{})
	if routes != nil {
		t.Errorf("expected nil routes for nil state, got %d", len(routes))
	}
}

func TestBuildTunnelRoutes_EmptyHostnameMap(t *testing.T) {
	state := &tunnel.TunnelState{
		HostnameMap: map[string]string{},
	}
	routes := BuildTunnelRoutes(state, map[string]registry.Allocation{})
	if routes != nil {
		t.Errorf("expected nil routes for empty hostname map, got %d", len(routes))
	}
}

func TestMergeTunnelRoutes(t *testing.T) {
	rt := &RouteTable{}
	// Set up base routes
	rt.update(map[string]route{
		"myapp.test": {Port: 14139},
	})

	// Merge tunnel routes on top
	rt.MergeTunnelRoutes(map[string]route{
		"abc123.trycloudflare.com": {Port: 14139, HostOverride: "myapp.test"},
	})

	// Original route should still exist
	r, ok := rt.Lookup("myapp.test")
	if !ok || r.Port != 14139 {
		t.Errorf("base route lost after merge: ok=%v port=%d", ok, r.Port)
	}

	// Tunnel route should exist
	r, ok = rt.Lookup("abc123.trycloudflare.com")
	if !ok {
		t.Fatal("expected tunnel route to exist")
	}
	if r.Port != 14139 || r.HostOverride != "myapp.test" {
		t.Errorf("tunnel route: got port=%d override=%q", r.Port, r.HostOverride)
	}
}

func TestRouteTableLookup(t *testing.T) {
	rt := &RouteTable{}
	rt.update(map[string]route{"myapp.test": {Port: 24920}})

	r, ok := rt.Lookup("myapp.test")
	if !ok {
		t.Fatal("expected lookup to succeed")
	}
	if r.Port != 24920 {
		t.Errorf("got %d, want 24920", r.Port)
	}

	_, ok = rt.Lookup("unknown.test")
	if ok {
		t.Error("expected lookup to fail for unknown host")
	}
}

func TestRouteTableUpdateReplacesRoutes(t *testing.T) {
	rt := &RouteTable{}
	rt.update(map[string]route{"old.test": {Port: 10000}})
	rt.update(map[string]route{"new.test": {Port: 20000}})

	_, ok := rt.Lookup("old.test")
	if ok {
		t.Error("old.test should not exist after update")
	}

	r, ok := rt.Lookup("new.test")
	if !ok {
		t.Fatal("expected new.test lookup to succeed")
	}
	if r.Port != 20000 {
		t.Errorf("got %d, want 20000", r.Port)
	}
}

// writeRegistryJSON writes a registry to a file using atomic temp+rename,
// matching how the real registry.Save() works.
func writeRegistryJSON(t *testing.T, path string, reg *registry.Registry) {
	t.Helper()
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatalf("rename: %v", err)
	}
}

func TestWatchAndRebuild(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Write initial registry with one project
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "app1.test"},
	})
	writeRegistryJSON(t, regPath, reg)

	rt := &RouteTable{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- WatchAndRebuild(ctx, regPath, rt)
	}()

	// Wait for initial load
	deadline := time.After(2 * time.Second)
	for {
		if _, ok := rt.Lookup("app1.test"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for initial route load")
		case err := <-errCh:
			t.Fatalf("watcher returned early: %v", err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	r, _ := rt.Lookup("app1.test")
	if r.Port != 10001 {
		t.Fatalf("app1.test: got %d, want 10001", r.Port)
	}

	// Update registry with a second project
	reg.Set("app2", "main", registry.Allocation{
		ProjectDir: "/src/app2",
		Ports:      map[string]int{"web": 10002},
		Hostnames:  map[string]string{"web": "app2.test"},
	})
	writeRegistryJSON(t, regPath, reg)

	// Wait for watcher to pick up the change
	deadline = time.After(2 * time.Second)
	for {
		if _, ok := rt.Lookup("app2.test"); ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for updated route")
		case err := <-errCh:
			t.Fatalf("watcher returned early: %v", err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	r2, _ := rt.Lookup("app2.test")
	if r2.Port != 10002 {
		t.Fatalf("app2.test: got %d, want 10002", r2.Port)
	}

	// Original route should still exist
	r3, ok := rt.Lookup("app1.test")
	if !ok {
		t.Fatal("app1.test should still exist")
	}
	if r3.Port != 10001 {
		t.Fatalf("app1.test: got %d, want 10001", r3.Port)
	}

	// Cancel and verify clean shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watcher returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
	}
}

func TestRouteTableAllocations(t *testing.T) {
	rt := &RouteTable{}
	allocs := map[string]registry.Allocation{
		"myapp/main": {
			ProjectDir: "/src/myapp",
			Ports:      map[string]int{"web": 10001, "postgres": 5432},
			Hostnames:  map[string]string{"web": "myapp.test"},
			},
	}
	rt.UpdateWithAllocations(map[string]route{"myapp.test": {Port: 10001}}, allocs)

	got := rt.Allocations()
	if len(got) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(got))
	}
	a, ok := got["myapp/main"]
	if !ok {
		t.Fatal("expected myapp/main allocation")
	}
	if a.Ports["web"] != 10001 {
		t.Errorf("web port: got %d, want 10001", a.Ports["web"])
	}
}

func TestRouteTableAllPorts(t *testing.T) {
	rt := &RouteTable{}

	// Empty route table returns nil.
	if ports := rt.AllPorts(); len(ports) != 0 {
		t.Fatalf("expected 0 ports on empty table, got %d", len(ports))
	}

	// After updating with allocations, ports are collected and deduplicated.
	allocs := map[string]registry.Allocation{
		"app1/main": {
			ProjectDir: "/src/app1",
			Ports:      map[string]int{"web": 10001, "postgres": 15432},
		},
		"app2/main": {
			ProjectDir: "/src/app2",
			Ports:      map[string]int{"web": 10002},
		},
	}
	rt.UpdateWithAllocations(nil, allocs)

	ports := rt.AllPorts()
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d: %v", len(ports), ports)
	}

	portSet := make(map[int]bool)
	for _, p := range ports {
		portSet[p] = true
	}
	for _, want := range []int{10001, 15432, 10002} {
		if !portSet[want] {
			t.Errorf("missing port %d in AllPorts: %v", want, ports)
		}
	}
}

func TestRouteTableAllPortsDeduplicates(t *testing.T) {
	rt := &RouteTable{}
	// Two allocations sharing a port value (unlikely in practice but tests dedup).
	allocs := map[string]registry.Allocation{
		"app1/main": {Ports: map[string]int{"web": 10001}},
		"app2/main": {Ports: map[string]int{"web": 10001}},
	}
	rt.UpdateWithAllocations(nil, allocs)

	ports := rt.AllPorts()
	if len(ports) != 1 {
		t.Fatalf("expected 1 deduplicated port, got %d: %v", len(ports), ports)
	}
	if ports[0] != 10001 {
		t.Errorf("expected port 10001, got %d", ports[0])
	}
}

func TestWatchAndRebuildMissingFileKeepsRoutes(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "registry.json")

	// Start with no file — should succeed (empty routes)
	rt := &RouteTable{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- WatchAndRebuild(ctx, regPath, rt)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// No routes should exist yet
	if _, ok := rt.Lookup("anything.test"); ok {
		t.Fatal("expected no routes with missing file")
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watcher returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher shutdown")
	}
}

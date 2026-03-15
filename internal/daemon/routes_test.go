package daemon

import (
	"testing"

	"github.com/outport-app/outport/internal/registry"
)

func TestBuildRoutes(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"rails": 24920, "postgres": 5432},
		Hostnames:  map[string]string{"rails": "myapp.test"},
		Protocols:  map[string]string{"rails": "http"},
	})

	routes := BuildRoutes(reg)
	if routes["myapp.test"] != 24920 {
		t.Errorf("myapp.test: got %d, want 24920", routes["myapp.test"])
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

func TestBuildRoutesSkipsNonHTTPProtocols(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"redis": 16379},
		Hostnames:  map[string]string{"redis": "redis.myapp.test"},
		Protocols:  map[string]string{"redis": "tcp"},
	})

	routes := BuildRoutes(reg)
	if len(routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes))
	}
}

func TestBuildRoutesIncludesHTTPS(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 24920},
		Hostnames:  map[string]string{"web": "myapp.test"},
		Protocols:  map[string]string{"web": "https"},
	})

	routes := BuildRoutes(reg)
	if routes["myapp.test"] != 24920 {
		t.Errorf("myapp.test: got %d, want 24920", routes["myapp.test"])
	}
}

func TestBuildRoutesMultipleProjects(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("app1", "main", registry.Allocation{
		ProjectDir: "/src/app1",
		Ports:      map[string]int{"web": 10001},
		Hostnames:  map[string]string{"web": "app1.test"},
		Protocols:  map[string]string{"web": "http"},
	})
	reg.Set("app2", "main", registry.Allocation{
		ProjectDir: "/src/app2",
		Ports:      map[string]int{"web": 10002},
		Hostnames:  map[string]string{"web": "app2.test"},
		Protocols:  map[string]string{"web": "http"},
	})

	routes := BuildRoutes(reg)
	if routes["app1.test"] != 10001 {
		t.Errorf("app1.test: got %d, want 10001", routes["app1.test"])
	}
	if routes["app2.test"] != 10002 {
		t.Errorf("app2.test: got %d, want 10002", routes["app2.test"])
	}
}

func TestRouteTableLookup(t *testing.T) {
	rt := &RouteTable{}
	rt.Update(map[string]int{"myapp.test": 24920})

	port, ok := rt.Lookup("myapp.test")
	if !ok {
		t.Fatal("expected lookup to succeed")
	}
	if port != 24920 {
		t.Errorf("got %d, want 24920", port)
	}

	_, ok = rt.Lookup("unknown.test")
	if ok {
		t.Error("expected lookup to fail for unknown host")
	}
}

func TestRouteTableUpdateReplacesRoutes(t *testing.T) {
	rt := &RouteTable{}
	rt.Update(map[string]int{"old.test": 10000})
	rt.Update(map[string]int{"new.test": 20000})

	_, ok := rt.Lookup("old.test")
	if ok {
		t.Error("old.test should not exist after update")
	}

	port, ok := rt.Lookup("new.test")
	if !ok {
		t.Fatal("expected new.test lookup to succeed")
	}
	if port != 20000 {
		t.Errorf("got %d, want 20000", port)
	}
}

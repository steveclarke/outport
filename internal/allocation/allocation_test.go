package allocation

import (
	"testing"

	"github.com/steveclarke/outport/internal/config"
)

func TestBuild(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails":    {EnvVar: "PORT", Hostname: "myapp.test"},
			"postgres": {EnvVar: "PGPORT"},
		},
	}
	ports := map[string]int{"rails": 24920, "postgres": 15432}

	alloc := Build(cfg, "main", "/src/myapp", ports)

	if alloc.ProjectDir != "/src/myapp" {
		t.Errorf("ProjectDir = %q, want /src/myapp", alloc.ProjectDir)
	}
	if alloc.Ports["rails"] != 24920 {
		t.Errorf("Ports[rails] = %d, want 24920", alloc.Ports["rails"])
	}
	if alloc.Hostnames["rails"] != "myapp.test" {
		t.Errorf("Hostnames[rails] = %q, want myapp.test", alloc.Hostnames["rails"])
	}
	if alloc.EnvVars["rails"] != "PORT" {
		t.Errorf("EnvVars[rails] = %q, want PORT", alloc.EnvVars["rails"])
	}
}

func TestComputeHostnames_Main(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {Hostname: "myapp.test"},
			"api":   {Hostname: "api-myapp.test"},
			"db":    {}, // no hostname
		},
	}

	hostnames := ComputeHostnames(cfg, "main")

	if hostnames["rails"] != "myapp.test" {
		t.Errorf("rails hostname = %q, want myapp.test", hostnames["rails"])
	}
	if hostnames["api"] != "api-myapp.test" {
		t.Errorf("api hostname = %q, want api-myapp.test", hostnames["api"])
	}
	if _, ok := hostnames["db"]; ok {
		t.Error("db should not have a hostname")
	}
}

func TestComputeHostnames_NonMainInstance(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {Hostname: "myapp.test"},
			"api":   {Hostname: "api-myapp.test"},
		},
	}

	hostnames := ComputeHostnames(cfg, "bxcf")

	if hostnames["rails"] != "myapp-bxcf.test" {
		t.Errorf("rails hostname = %q, want myapp-bxcf.test", hostnames["rails"])
	}
	if hostnames["api"] != "api-myapp-bxcf.test" {
		t.Errorf("api hostname = %q, want api-myapp-bxcf.test", hostnames["api"])
	}
}

func TestBuildTemplateVars_Basic(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT"},
		},
	}
	ports := map[string]int{"rails": 24920}
	hostnames := map[string]string{"rails": "myapp.test"}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, false, nil)

	if vars["project_name"] != "myapp" {
		t.Errorf("project_name = %q", vars["project_name"])
	}
	if vars["instance"] != "" {
		t.Errorf("instance = %q, want empty for main", vars["instance"])
	}
	if vars["rails.port"] != "24920" {
		t.Errorf("rails.port = %q", vars["rails.port"])
	}
	if vars["rails.hostname"] != "myapp.test" {
		t.Errorf("rails.hostname = %q", vars["rails.hostname"])
	}
	if vars["rails.url"] != "http://myapp.test" {
		t.Errorf("rails.url = %q, want http://myapp.test", vars["rails.url"])
	}
}

func TestBuildTemplateVars_HTTPS(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT"},
		},
	}
	ports := map[string]int{"rails": 24920}
	hostnames := map[string]string{"rails": "myapp.test"}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, true, nil)

	if vars["rails.url"] != "https://myapp.test" {
		t.Errorf("rails.url = %q, want https://myapp.test", vars["rails.url"])
	}
}

func TestBuildTemplateVars_Instance(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT"},
		},
	}
	ports := map[string]int{"rails": 24920}

	vars := BuildTemplateVars(cfg, "bxcf", ports, nil, false, nil)

	if vars["instance"] != "bxcf" {
		t.Errorf("instance = %q, want bxcf", vars["instance"])
	}
}

func TestBuildTemplateVars_TunnelOverride(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {EnvVar: "PORT"},
		},
	}
	ports := map[string]int{"rails": 24920}
	hostnames := map[string]string{"rails": "myapp.test"}
	tunnelURLs := map[string]string{"rails": "https://abc123.trycloudflare.com"}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, true, tunnelURLs)

	if vars["rails.url"] != "https://abc123.trycloudflare.com" {
		t.Errorf("rails.url = %q, want tunnel URL", vars["rails.url"])
	}
	if vars["rails.url:direct"] != "http://localhost:24920" {
		t.Errorf("rails.url:direct = %q, want localhost", vars["rails.url:direct"])
	}
}

func TestResolveComputed_Empty(t *testing.T) {
	cfg := &config.Config{
		Name:     "myapp",
		Services: map[string]config.Service{},
	}

	result := ResolveComputed(cfg, "main", nil, nil, false, nil)
	if result != nil {
		t.Errorf("expected nil for empty computed, got %v", result)
	}
}

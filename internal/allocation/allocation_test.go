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

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, nil, false, nil)

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

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, nil, true, nil)

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

	vars := BuildTemplateVars(cfg, "bxcf", ports, nil, nil, false, nil)

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

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, nil, true, tunnelURLs)

	if vars["rails.url"] != "https://abc123.trycloudflare.com" {
		t.Errorf("rails.url = %q, want tunnel URL", vars["rails.url"])
	}
	if vars["rails.url:direct"] != "http://localhost:24920" {
		t.Errorf("rails.url:direct = %q, want localhost", vars["rails.url:direct"])
	}
}

func TestComputeAliases_Main(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				Hostname: "approvethis.test",
				Aliases:  map[string]string{"app": "app.approvethis.test", "admin": "admin.approvethis.test"},
			},
			"db": {EnvVar: "PGPORT"}, // no hostname or aliases
		},
	}

	aliases := ComputeAliases(cfg, "main")

	if len(aliases) != 1 {
		t.Fatalf("expected 1 service with aliases, got %d", len(aliases))
	}
	if aliases["web"]["app"] != "app.approvethis.test" {
		t.Errorf("web/app = %q, want app.approvethis.test", aliases["web"]["app"])
	}
	if aliases["web"]["admin"] != "admin.approvethis.test" {
		t.Errorf("web/admin = %q, want admin.approvethis.test", aliases["web"]["admin"])
	}
}

func TestComputeAliases_NonMainInstance(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				Hostname: "approvethis.test",
				Aliases:  map[string]string{"app": "app.approvethis.test"},
			},
		},
	}

	aliases := ComputeAliases(cfg, "bxcf")

	if aliases["web"]["app"] != "app.approvethis-bxcf.test" {
		t.Errorf("web/app = %q, want app.approvethis-bxcf.test", aliases["web"]["app"])
	}
}

func TestComputeAliases_NoAliases(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"web": {Hostname: "myapp.test"},
		},
	}

	aliases := ComputeAliases(cfg, "main")

	if len(aliases) != 0 {
		t.Errorf("expected 0 services with aliases, got %d", len(aliases))
	}
}

func TestComputeAliases_WithTestSuffix(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				Hostname: "approvethis.test",
				Aliases:  map[string]string{"app": "app.approvethis.test"},
			},
		},
	}

	aliases := ComputeAliases(cfg, "main")

	if aliases["web"]["app"] != "app.approvethis.test" {
		t.Errorf("web/app = %q, want app.approvethis.test", aliases["web"]["app"])
	}
}

func TestBuildTemplateVars_Aliases(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				EnvVar:   "PORT",
				Hostname: "approvethis.test",
				Aliases:  map[string]string{"app": "app.approvethis.test"},
			},
		},
	}
	ports := map[string]int{"web": 14139}
	hostnames := map[string]string{"web": "approvethis.test"}
	aliases := map[string]map[string]string{
		"web": {"app": "app.approvethis.test"},
	}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, aliases, true, nil)

	if vars["web.alias.app"] != "app.approvethis.test" {
		t.Errorf("web.alias.app = %q, want app.approvethis.test", vars["web.alias.app"])
	}
	if vars["web.alias_url.app"] != "https://app.approvethis.test" {
		t.Errorf("web.alias_url.app = %q, want https://app.approvethis.test", vars["web.alias_url.app"])
	}
}

func TestBuildTemplateVars_AliasesWithTunnel(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				EnvVar:   "PORT",
				Hostname: "approvethis.test",
				Aliases:  map[string]string{"app": "app.approvethis.test"},
			},
		},
	}
	ports := map[string]int{"web": 14139}
	hostnames := map[string]string{"web": "approvethis.test"}
	aliases := map[string]map[string]string{
		"web": {"app": "app.approvethis.test"},
	}
	tunnelURLs := map[string]string{
		"web":           "https://abc123.trycloudflare.com",
		"web/alias/app": "https://def456.trycloudflare.com",
	}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, aliases, true, tunnelURLs)

	if vars["web.alias_url.app"] != "https://def456.trycloudflare.com" {
		t.Errorf("web.alias_url.app = %q, want tunnel URL", vars["web.alias_url.app"])
	}
}

func TestResolveComputed_Empty(t *testing.T) {
	cfg := &config.Config{
		Name:     "myapp",
		Services: map[string]config.Service{},
	}

	result := ResolveComputed(cfg, "main", nil, nil, nil, false, nil)
	if result != nil {
		t.Errorf("expected nil for empty computed, got %v", result)
	}
}

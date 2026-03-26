# Hostname Aliases Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support multiple named hostname aliases per service, so a single port can be reached via multiple `.test` domains with full template, CLI, dashboard, and tunnel support.

**Architecture:** Named aliases (`map[string]string`) on the config `Service` struct flow through allocation into the registry, where the daemon's route builder maps every alias hostname to the same port as its primary. The template parser is extended to handle compound fields (`alias.NAME`, `alias_url.NAME`). Tunnels are rearchitected to route through the proxy with Host header rewriting so host-based routing works correctly.

**Tech Stack:** Go, YAML (gopkg.in/yaml.v3), go-ini/ini, Cobra CLI, httputil.ReverseProxy

**Spec:** `docs/superpowers/specs/2026-03-26-hostname-aliases-design.md`

---

## File Map

### Modified files

| File | Responsibility |
|---|---|
| `internal/config/config.go` | Add `Aliases` field to `Service` and `rawService`, validate alias keys and hostnames |
| `internal/config/config_test.go` | Tests for alias config parsing and validation |
| `internal/config/expand.go` | No changes needed — `ExpandVars` already handles dotted keys like `web.alias.app` |
| `internal/allocation/allocation.go` | Add `ComputeAliases`, extend `Build` and `BuildTemplateVars` for alias variables |
| `internal/allocation/allocation_test.go` | Tests for `ComputeAliases` and alias template vars |
| `internal/registry/registry.go` | Add `Aliases` field to `Allocation`, extend `FindHostname` and `Load` |
| `internal/registry/registry_test.go` | Tests for alias storage and hostname conflict detection |
| `internal/daemon/routes.go` | Change route map to `map[string]route` struct, add `HostOverride`, build alias routes |
| `internal/daemon/routes_test.go` | Tests for alias routing and `HostOverride` |
| `internal/daemon/proxy.go` | Change `Lookup` call to return `route`, apply `HostOverride` to request |
| `internal/daemon/proxy_test.go` | Test Host header rewriting for tunnel routes |
| `internal/dashboard/handler.go` | Add `Aliases` to `ServiceJSON`, populate in `buildStatus` |
| `cmd/render.go` | Add `Aliases` to `svcJSON`, populate in `buildServiceMap`, show all URLs in styled output |
| `cmd/up.go` | Validate alias hostname uniqueness, pass aliases through allocation |
| `cmd/envfiles.go` | Pass aliases to `BuildTemplateVars` (via allocation) |
| `cmd/share.go` | Build per-hostname tunnel map, route through proxy, add tunnel routes |
| `internal/settings/settings.go` | Add `Tunnels.Max` setting |
| `internal/settings/settings_test.go` | Test `max_tunnels` setting |

---

## Task 1: Config — Parse and Validate Aliases

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for alias parsing**

Add to `internal/config/config_test.go`:

```go
func TestLoad_ServiceAliases(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
      admin: admin.approvethis
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	web := cfg.Services["web"]
	if len(web.Aliases) != 2 {
		t.Fatalf("aliases count = %d, want 2", len(web.Aliases))
	}
	if web.Aliases["app"] != "app.approvethis" {
		t.Errorf("alias app = %q, want %q", web.Aliases["app"], "app.approvethis")
	}
	if web.Aliases["admin"] != "admin.approvethis" {
		t.Errorf("alias admin = %q, want %q", web.Aliases["admin"], "admin.approvethis")
	}
}

func TestLoad_AliasesWithTestSuffix(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis.test
    aliases:
      app: app.approvethis.test
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Aliases["app"] != "app.approvethis.test" {
		t.Errorf("alias should preserve .test suffix from config")
	}
}

func TestLoad_NoAliases(t *testing.T) {
	dir := writeConfig(t, `name: myapp
services:
  web:
    env_var: PORT
    hostname: myapp
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Aliases != nil {
		t.Errorf("aliases should be nil when not specified")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_ServiceAliases|TestLoad_AliasesWithTestSuffix|TestLoad_NoAliases" -v`
Expected: FAIL — `Service` has no `Aliases` field.

- [ ] **Step 3: Add Aliases field to Service and rawService structs**

In `internal/config/config.go`, add the `Aliases` field to `Service`:

```go
type Service struct {
	PreferredPort int               `yaml:"preferred_port"`
	EnvVar        string            `yaml:"env_var"`
	Hostname      string            `yaml:"hostname"`
	Aliases       map[string]string `yaml:"aliases"`
	rawEnvFile    envFileField
	EnvFiles      []string `yaml:"-"`
}
```

Add to `rawService`:

```go
type rawService struct {
	PreferredPort int               `yaml:"preferred_port"`
	EnvVar        string            `yaml:"env_var"`
	Hostname      string            `yaml:"hostname"`
	Aliases       map[string]string `yaml:"aliases"`
	EnvFile       envFileField      `yaml:"env_file"`
}
```

Update `toService` to copy aliases:

```go
func toService(rs rawService) Service {
	return Service{
		PreferredPort: rs.PreferredPort,
		EnvVar:        rs.EnvVar,
		Hostname:      rs.Hostname,
		Aliases:       rs.Aliases,
		rawEnvFile:    rs.EnvFile,
	}
}
```

- [ ] **Step 4: Run tests to verify parsing works**

Run: `go test ./internal/config/ -run "TestLoad_ServiceAliases|TestLoad_AliasesWithTestSuffix|TestLoad_NoAliases" -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for alias validation**

Add to `internal/config/config_test.go`:

```go
func TestValidateAliasKeyFormat(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid lowercase", "app", false},
		{"valid with hyphens", "my-app", false},
		{"valid with digits", "app2", false},
		{"invalid uppercase", "App", true},
		{"invalid underscore", "my_app", true},
		{"invalid dot", "my.app", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      %s: app.approvethis
`, tt.key)
			dir := writeConfig(t, yaml)
			_, err := Load(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("alias key %q: err=%v, wantErr=%v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAliasHostnameRules(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantErr  bool
	}{
		{"valid subdomain", "app.approvethis", false},
		{"valid with test suffix", "app.approvethis.test", false},
		{"invalid chars", "app_approvethis", true},
		{"missing project name", "app.other", true},
		{"reserved outport.test", "outport.test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: %s
`, tt.hostname)
			dir := writeConfig(t, yaml)
			_, err := Load(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("alias hostname %q: err=%v, wantErr=%v", tt.hostname, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAliasDuplicatesOwnHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      dupe: approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when alias duplicates primary hostname")
	}
	if !strings.Contains(err.Error(), "conflicts with service's own hostname") {
		t.Errorf("error = %v, want mention of own hostname conflict", err)
	}
}

func TestValidateAliasDuplicatesAnotherAlias(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
      dupe: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when two aliases share a hostname")
	}
}

func TestValidateAliasWithoutPrimaryHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    aliases:
      app: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when aliases defined without primary hostname")
	}
}

func TestValidateAliasDuplicatesAnotherServiceHostname(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: api.approvethis
  api:
    env_var: API_PORT
    hostname: api.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when alias duplicates another service's hostname")
	}
}

func TestValidateAliasDuplicatesAnotherServiceAlias(t *testing.T) {
	dir := writeConfig(t, `
name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
  api:
    env_var: API_PORT
    hostname: api.approvethis
    aliases:
      dupe: app.approvethis
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when aliases across services share a hostname")
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestValidateAlias" -v`
Expected: FAIL — no alias validation logic yet.

- [ ] **Step 7: Implement alias validation in config.go**

Add an alias key regex near the top of `config.go`:

```go
var aliasKeyRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
```

Add alias validation to the `validate()` method, after the existing hostname validation block (after line 442). This replaces the closing of the hostname validation loop — the alias validation goes inside the same `for name, svc := range c.Services` loop that validates hostnames:

```go
	// Collect all hostnames for cross-service duplicate detection.
	// Maps hostname stem -> "service:name" or "service:name/alias:key"
	allHostnames := make(map[string]string)
	for name, svc := range c.Services {
		if svc.Hostname != "" {
			stem := strings.TrimSuffix(svc.Hostname, ".test")
			allHostnames[stem] = fmt.Sprintf("service %q", name)
		}
		for key, aliasHostname := range svc.Aliases {
			stem := strings.TrimSuffix(aliasHostname, ".test")
			label := fmt.Sprintf("service %q alias %q", name, key)
			if existing, ok := allHostnames[stem]; ok {
				return fmt.Errorf("%s: hostname %q conflicts with %s", label, aliasHostname, existing)
			}
			allHostnames[stem] = label
		}
	}
```

Then inside the existing `for name, svc := range c.Services` hostname validation loop, add alias validation after the primary hostname checks:

```go
		if len(svc.Aliases) > 0 && svc.Hostname == "" {
			return fmt.Errorf("service %q: aliases require a primary hostname", name)
		}

		for key, aliasHostname := range svc.Aliases {
			if !aliasKeyRe.MatchString(key) {
				return fmt.Errorf("service %q: alias key %q is invalid (must be lowercase alphanumeric with hyphens)", name, key)
			}

			if aliasHostname == "outport.test" {
				return fmt.Errorf("service %q: alias %q hostname %q is reserved for the Outport dashboard", name, key, aliasHostname)
			}

			aliasStem := strings.TrimSuffix(aliasHostname, ".test")
			if !hostnameRe.MatchString(aliasStem) {
				return fmt.Errorf("service %q: alias %q hostname %q contains invalid characters (use lowercase alphanumeric, hyphens, dots)", name, key, aliasHostname)
			}
			if !strings.Contains(aliasStem, c.Name) {
				return fmt.Errorf("service %q: alias %q hostname %q must contain project name %q", name, key, aliasHostname, c.Name)
			}

			// Check alias doesn't duplicate own primary hostname
			primaryStem := strings.TrimSuffix(svc.Hostname, ".test")
			if aliasStem == primaryStem {
				return fmt.Errorf("service %q: alias %q hostname conflicts with service's own hostname", name, key)
			}
		}
```

- [ ] **Step 8: Run all alias validation tests**

Run: `go test ./internal/config/ -run "TestValidateAlias|TestLoad_ServiceAliases|TestLoad_AliasesWithTestSuffix|TestLoad_NoAliases" -v`
Expected: PASS

- [ ] **Step 9: Run full config test suite to check for regressions**

Run: `go test ./internal/config/ -v`
Expected: PASS — all existing tests still pass.

- [ ] **Step 10: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: parse and validate hostname aliases in config"
```

---

## Task 2: Allocation — ComputeAliases and Template Variables

**Files:**
- Modify: `internal/allocation/allocation.go`
- Test: `internal/allocation/allocation_test.go`

- [ ] **Step 1: Write failing tests for ComputeAliases**

Add to `internal/allocation/allocation_test.go`:

```go
func TestComputeAliases_Main(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				Hostname: "approvethis",
				Aliases:  map[string]string{"app": "app.approvethis", "admin": "admin.approvethis"},
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
				Hostname: "approvethis",
				Aliases:  map[string]string{"app": "app.approvethis"},
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
			"web": {Hostname: "myapp"},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/allocation/ -run "TestComputeAliases" -v`
Expected: FAIL — `ComputeAliases` does not exist.

- [ ] **Step 3: Implement ComputeAliases**

Add to `internal/allocation/allocation.go`:

```go
// ComputeAliases builds a map of service name -> alias name -> .test hostname
// for every service that declares aliases in the config. Instance suffixing
// follows the same rules as primary hostnames.
func ComputeAliases(cfg *config.Config, instanceName string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for svcName, svc := range cfg.Services {
		if len(svc.Aliases) == 0 {
			continue
		}
		svcAliases := make(map[string]string)
		for key, aliasHostname := range svc.Aliases {
			stem := strings.TrimSuffix(aliasHostname, ".test")
			if instanceName != "main" {
				idx := strings.LastIndex(stem, cfg.Name)
				if idx >= 0 {
					stem = stem[:idx] + cfg.Name + "-" + instanceName + stem[idx+len(cfg.Name):]
				}
			}
			svcAliases[key] = stem + ".test"
		}
		result[svcName] = svcAliases
	}
	return result
}
```

- [ ] **Step 4: Run ComputeAliases tests**

Run: `go test ./internal/allocation/ -run "TestComputeAliases" -v`
Expected: PASS

- [ ] **Step 5: Update Build to include aliases**

Modify `Build` in `internal/allocation/allocation.go`:

```go
func Build(cfg *config.Config, instanceName, dir string, ports map[string]int) registry.Allocation {
	return registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
		Hostnames:  ComputeHostnames(cfg, instanceName),
		Aliases:    ComputeAliases(cfg, instanceName),
		EnvVars:    computeEnvVars(cfg),
	}
}
```

Note: This requires the `Aliases` field on `registry.Allocation` — that will be added in Task 3. The code won't compile until both tasks are done. If building incrementally, add the registry field first or accept the compile error until Task 3.

- [ ] **Step 6: Write failing test for alias template variables**

Add to `internal/allocation/allocation_test.go`:

```go
func TestBuildTemplateVars_Aliases(t *testing.T) {
	cfg := &config.Config{
		Name: "approvethis",
		Services: map[string]config.Service{
			"web": {
				EnvVar:   "PORT",
				Hostname: "approvethis",
				Aliases:  map[string]string{"app": "app.approvethis"},
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
				Hostname: "approvethis",
				Aliases:  map[string]string{"app": "app.approvethis"},
			},
		},
	}
	ports := map[string]int{"web": 14139}
	hostnames := map[string]string{"web": "approvethis.test"}
	aliases := map[string]map[string]string{
		"web": {"app": "app.approvethis.test"},
	}
	tunnelURLs := map[string]string{
		"web":          "https://abc123.trycloudflare.com",
		"web/alias/app": "https://def456.trycloudflare.com",
	}

	vars := BuildTemplateVars(cfg, "main", ports, hostnames, aliases, true, tunnelURLs)

	if vars["web.alias_url.app"] != "https://def456.trycloudflare.com" {
		t.Errorf("web.alias_url.app = %q, want tunnel URL", vars["web.alias_url.app"])
	}
}
```

- [ ] **Step 7: Run tests to verify they fail**

Run: `go test ./internal/allocation/ -run "TestBuildTemplateVars_Aliases" -v`
Expected: FAIL — `BuildTemplateVars` doesn't accept aliases parameter.

- [ ] **Step 8: Extend BuildTemplateVars to include alias variables**

Update the signature and body of `BuildTemplateVars` in `internal/allocation/allocation.go`. Add an `aliases` parameter:

```go
func BuildTemplateVars(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]string {
	vars := make(map[string]string)
	vars["project_name"] = cfg.Name
	if instanceName == "main" {
		vars["instance"] = ""
	} else {
		vars["instance"] = instanceName
	}
	for name, svc := range cfg.Services {
		portStr := fmt.Sprintf("%d", ports[name])
		vars[name+".port"] = portStr
		vars[name+".env_var"] = svc.EnvVar

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h

			if tunnelURL, hasTunnel := tunnelURLs[name]; hasTunnel {
				vars[name+".url"] = tunnelURL
			} else {
				vars[name+".url"] = fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(h, httpsEnabled), h)
			}
			vars[name+".url:direct"] = fmt.Sprintf("http://localhost:%s", portStr)
		} else {
			hostname := svc.Hostname
			if hostname == "" {
				hostname = "localhost"
			}
			vars[name+".hostname"] = hostname
		}

		// Alias template variables
		if svcAliases, ok := aliases[name]; ok {
			for key, aliasHostname := range svcAliases {
				vars[name+".alias."+key] = aliasHostname

				tunnelKey := name + "/alias/" + key
				if tunnelURL, hasTunnel := tunnelURLs[tunnelKey]; hasTunnel {
					vars[name+".alias_url."+key] = tunnelURL
				} else {
					vars[name+".alias_url."+key] = fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(aliasHostname, httpsEnabled), aliasHostname)
				}
			}
		}
	}
	return vars
}
```

- [ ] **Step 9: Update ResolveComputed to pass aliases**

Update `ResolveComputed` signature to accept and pass aliases:

```go
func ResolveComputed(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]map[string]string {
	if len(cfg.Computed) == 0 {
		return nil
	}
	templateVars := BuildTemplateVars(cfg, instanceName, ports, hostnames, aliases, httpsEnabled, tunnelURLs)
	return config.ResolveComputed(cfg.Computed, templateVars)
}
```

- [ ] **Step 10: Fix existing tests that call BuildTemplateVars and ResolveComputed**

Update all existing test calls in `internal/allocation/allocation_test.go` to pass `nil` for the new `aliases` parameter. For example:

`BuildTemplateVars(cfg, "main", ports, hostnames, false, nil)` becomes `BuildTemplateVars(cfg, "main", ports, hostnames, nil, false, nil)`

`ResolveComputed(cfg, "main", nil, nil, false, nil)` becomes `ResolveComputed(cfg, "main", nil, nil, nil, false, nil)`

- [ ] **Step 11: Run all allocation tests**

Run: `go test ./internal/allocation/ -v`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add internal/allocation/allocation.go internal/allocation/allocation_test.go
git commit -m "feat: compute alias hostnames and template variables"
```

---

## Task 3: Registry — Store and Search Aliases

**Files:**
- Modify: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/registry/registry_test.go`:

```go
func TestFindHostname_Aliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	reg, _ := Load(path)

	reg.Set("approvethis", "main", Allocation{
		ProjectDir: "/src/approvethis",
		Ports:      map[string]int{"web": 14139},
		Hostnames:  map[string]string{"web": "approvethis.test"},
		Aliases: map[string]map[string]string{
			"web": {"app": "app.approvethis.test"},
		},
	})

	// Should find alias hostname
	key, found := reg.FindHostname("app.approvethis.test", "other/main")
	if !found {
		t.Fatal("expected to find alias hostname")
	}
	if key != "approvethis/main" {
		t.Errorf("key = %q, want approvethis/main", key)
	}

	// Self-exclude should skip alias
	_, found = reg.FindHostname("app.approvethis.test", "approvethis/main")
	if found {
		t.Error("expected self-exclude to skip alias")
	}
}

func TestRegistry_SaveAndReloadAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	reg, _ := Load(path)
	reg.Set("approvethis", "main", Allocation{
		ProjectDir: "/src/approvethis",
		Ports:      map[string]int{"web": 14139},
		Hostnames:  map[string]string{"web": "approvethis.test"},
		Aliases: map[string]map[string]string{
			"web": {"app": "app.approvethis.test"},
		},
	})
	if err := reg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	reg2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, ok := reg2.Get("approvethis", "main")
	if !ok {
		t.Fatal("allocation lost after reload")
	}
	if got.Aliases["web"]["app"] != "app.approvethis.test" {
		t.Errorf("alias lost after reload: %v", got.Aliases)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/registry/ -run "TestFindHostname_Aliases|TestRegistry_SaveAndReloadAliases" -v`
Expected: FAIL — `Allocation` has no `Aliases` field.

- [ ] **Step 3: Add Aliases field to Allocation and update FindHostname**

In `internal/registry/registry.go`, add the `Aliases` field to `Allocation`:

```go
type Allocation struct {
	ProjectDir            string                       `json:"project_dir"`
	Ports                 map[string]int               `json:"ports"`
	Hostnames             map[string]string            `json:"hostnames,omitempty"`
	Aliases               map[string]map[string]string `json:"aliases,omitempty"`
	EnvVars               map[string]string            `json:"env_vars,omitempty"`
	ApprovedExternalFiles []string                     `json:"approved_external_files,omitempty"`
}
```

Update `FindHostname` to also search aliases:

```go
func (r *Registry) FindHostname(hostname, excludeKey string) (string, bool) {
	for key, alloc := range r.Projects {
		if key == excludeKey {
			continue
		}
		for _, h := range alloc.Hostnames {
			if h == hostname {
				return key, true
			}
		}
		for _, svcAliases := range alloc.Aliases {
			for _, h := range svcAliases {
				if h == hostname {
					return key, true
				}
			}
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_test.go
git commit -m "feat: store and search aliases in registry"
```

---

## Task 4: Daemon Routes — Route Struct and Alias Routes

**Files:**
- Modify: `internal/daemon/routes.go`
- Test: `internal/daemon/routes_test.go`

- [ ] **Step 1: Write failing test for alias routes**

Add to `internal/daemon/routes_test.go`:

```go
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

	// Primary
	if routes["approvethis.test"].Port != 14139 {
		t.Errorf("primary: got %d, want 14139", routes["approvethis.test"].Port)
	}
	// Aliases point to same port
	if routes["app.approvethis.test"].Port != 14139 {
		t.Errorf("alias app: got %d, want 14139", routes["app.approvethis.test"].Port)
	}
	if routes["admin.approvethis.test"].Port != 14139 {
		t.Errorf("alias admin: got %d, want 14139", routes["admin.approvethis.test"].Port)
	}
	// No HostOverride for normal routes
	if routes["approvethis.test"].HostOverride != "" {
		t.Errorf("primary should have empty HostOverride")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run "TestBuildRoutesIncludesAliases" -v`
Expected: FAIL — `BuildRoutes` returns `map[string]int`, not `map[string]route`.

- [ ] **Step 3: Change route table from map[string]int to map[string]route**

In `internal/daemon/routes.go`, add the `route` struct and change all types:

```go
// route represents a single proxy route entry. Normal routes just carry a
// port; tunnel routes also carry a HostOverride so the proxy can rewrite the
// Host header to the original .test hostname before forwarding.
type route struct {
	Port         int
	HostOverride string // empty for normal routes, set for tunnel routes
}
```

Change `RouteTable`:

```go
type RouteTable struct {
	mu          sync.RWMutex
	routes      map[string]route
	allocations map[string]registry.Allocation
	ports       []int
	OnUpdate    func()
}
```

Update `Lookup` to return a `route`:

```go
func (rt *RouteTable) Lookup(hostname string) (route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	r, ok := rt.routes[hostname]
	return r, ok
}
```

Update `update` (used in tests):

```go
func (rt *RouteTable) update(routes map[string]route) {
	rt.mu.Lock()
	rt.routes = routes
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}
```

Update `UpdateWithAllocations`:

```go
func (rt *RouteTable) UpdateWithAllocations(routes map[string]route, allocs map[string]registry.Allocation) {
	rt.mu.Lock()
	rt.routes = routes
	rt.allocations = allocs
	rt.ports = deduplicatePorts(allocs)
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}
```

Update `BuildRoutes` to return `map[string]route` and include aliases:

```go
func BuildRoutes(reg *registry.Registry) map[string]route {
	routes := make(map[string]route)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			routes[hostname] = route{Port: alloc.Ports[svcName]}
		}
		for svcName, svcAliases := range alloc.Aliases {
			for _, aliasHostname := range svcAliases {
				routes[aliasHostname] = route{Port: alloc.Ports[svcName]}
			}
		}
	}
	return routes
}
```

- [ ] **Step 4: Fix existing route tests to use the new route struct**

Update all test calls that use `map[string]int` to use `map[string]route`. For example:

`rt.update(map[string]int{"myapp.test": 24920})` becomes `rt.update(map[string]route{"myapp.test": {Port: 24920}})`

And update assertions: `routes["myapp.test"]` (which was `int`) becomes `routes["myapp.test"].Port`.

For `Lookup` calls that return `(int, bool)`, update to `(route, bool)` and access `.Port`.

- [ ] **Step 5: Run all daemon tests**

Run: `go test ./internal/daemon/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/daemon/routes.go internal/daemon/routes_test.go
git commit -m "feat: route table supports aliases and HostOverride struct"
```

---

## Task 5: Proxy — Host Header Rewriting

**Files:**
- Modify: `internal/daemon/proxy.go`
- Test: `internal/daemon/proxy_test.go`

- [ ] **Step 1: Write failing test for Host header rewriting**

Add to `internal/daemon/proxy_test.go`:

```go
func TestProxyHostOverrideRewritesHostHeader(t *testing.T) {
	var gotHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	routes := &RouteTable{}
	routes.update(map[string]route{
		"abc123.trycloudflare.com": {Port: port, HostOverride: "myapp.test"},
	})

	proxy := NewProxy(routes)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "abc123.trycloudflare.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if gotHost != "myapp.test" {
		t.Errorf("backend saw Host %q, want %q", gotHost, "myapp.test")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/ -run "TestProxyHostOverrideRewritesHostHeader" -v`
Expected: FAIL — proxy doesn't handle `HostOverride`.

- [ ] **Step 3: Update proxy to use route struct and apply HostOverride**

In `internal/daemon/proxy.go`, update `ServeHTTP` to work with the `route` struct:

```go
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.Host
	if idx := strings.LastIndex(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	// Dashboard intercept
	if hostname == "outport.test" && p.DashboardHandler != nil {
		p.DashboardHandler.ServeHTTP(w, r)
		return
	}

	rt, ok := p.routes.Lookup(hostname)
	if !ok {
		writeErrorPage(w, http.StatusBadGateway, hostname,
			"No project is configured for this hostname.<br>Add a matching hostname to your <code>outport.yml</code> and run:",
			`<div class="hint">outport up</div>`)
		return
	}

	// Rewrite Host header for tunnel routes so the backend sees the original .test hostname
	if rt.HostOverride != "" {
		r.Host = rt.HostOverride
	}

	proxy := p.getOrCreateProxy(rt.Port)
	displayHostname := hostname
	if rt.HostOverride != "" {
		displayHostname = rt.HostOverride
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeErrorPage(w, http.StatusBadGateway, displayHostname,
			"This app isn't running yet.<br>Start your app, then refresh this page.",
			"")
	}
	proxy.ServeHTTP(w, r)
}
```

- [ ] **Step 4: Fix existing proxy tests**

Update all existing proxy test calls that use `map[string]int` to use `map[string]route`. For example:

`routes.update(map[string]int{"myapp.test": port})` becomes `routes.update(map[string]route{"myapp.test": {Port: port}})`

- [ ] **Step 5: Run all proxy tests**

Run: `go test ./internal/daemon/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/daemon/proxy.go internal/daemon/proxy_test.go
git commit -m "feat: proxy rewrites Host header for tunnel routes"
```

---

## Task 6: Template Validation — Alias References in Computed Values

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for alias template references**

Add to `internal/config/config_test.go`:

```go
func TestValidateTemplateRefAlias(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  APP_URL:
    value: "${web.alias_url.app}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected valid alias template ref, got: %v", err)
	}
}

func TestValidateTemplateRefAliasUnknown(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  BAD:
    value: "${web.alias.missing}"
    env_file: .env
`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown alias name in template")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want mention of missing alias", err)
	}
}

func TestValidateTemplateRefAliasURL(t *testing.T) {
	dir := writeConfig(t, `name: approvethis
services:
  web:
    env_var: PORT
    hostname: approvethis
    aliases:
      app: app.approvethis
computed:
  APP_HOSTNAME:
    value: "${web.alias.app}"
    env_file: .env
`)
	_, err := Load(dir)
	if err != nil {
		t.Fatalf("expected valid alias hostname template ref, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestValidateTemplateRefAlias" -v`
Expected: FAIL — `alias_url` and `alias` are not recognized as valid fields.

- [ ] **Step 3: Extend template validation for alias references**

The current `templateVarRe` regex is `\$\{(\w+)\.(\w+)(?::(\w+))?\}` which matches `${service.field}` or `${service.field:modifier}`. It won't match `${web.alias.app}` or `${web.alias_url.app}` because those have three dot-separated segments.

Add a new regex for alias template references and update `validateTemplateRefs` in `internal/config/config.go`:

```go
// aliasVarRe matches ${service.alias.name} and ${service.alias_url.name} references.
var aliasVarRe = regexp.MustCompile(`\$\{(\w+)\.(alias|alias_url)\.(\w+)\}`)
```

Then in `validateTemplateRefs`, add alias validation before the return:

```go
	// Validate ${service.alias.name} and ${service.alias_url.name} references
	aliasMatches := aliasVarRe.FindAllStringSubmatch(template, -1)
	for _, m := range aliasMatches {
		svcName := m[1]
		aliasName := m[3]

		svc, ok := services[svcName]
		if !ok {
			return fmt.Errorf("computed %q: references unknown service %q", computedName, svcName)
		}
		if _, ok := svc.Aliases[aliasName]; !ok {
			return fmt.Errorf("computed %q: service %q has no alias %q", computedName, svcName, aliasName)
		}
	}
```

Also, the existing `templateVarRe` will match the first two segments of `${web.alias.app}` as service=`web` field=`alias`, which would fail the `validFields` check. We need to exclude alias patterns from the general regex match. The simplest fix: add `alias` and `alias_url` as recognized fields that skip the normal validation:

In the existing `templateVarRe` match loop, add a skip for alias fields:

```go
	for _, m := range matches {
		svcName := m[1]
		field := m[2]
		modifier := ""
		if len(m) > 3 {
			modifier = m[3]
		}

		// Skip alias fields — they're handled by aliasVarRe
		if field == "alias" || field == "alias_url" {
			continue
		}

		if _, ok := services[svcName]; !ok {
			return fmt.Errorf("computed %q: references unknown service %q", computedName, svcName)
		}
		// ... rest unchanged
	}
```

- [ ] **Step 4: Run alias template tests**

Run: `go test ./internal/config/ -run "TestValidateTemplateRefAlias" -v`
Expected: PASS

- [ ] **Step 5: Run full config test suite**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: validate alias template references in computed values"
```

---

## Task 7: Dashboard — Display Aliases in API

**Files:**
- Modify: `internal/dashboard/handler.go`

- [ ] **Step 1: Add aliases to ServiceJSON and buildStatus**

In `internal/dashboard/handler.go`, add an `AliasJSON` struct and an `Aliases` field to `ServiceJSON`:

```go
// AliasJSON describes a single hostname alias for a service.
type AliasJSON struct {
	Hostname  string `json:"hostname"`
	URL       string `json:"url,omitempty"`
	TunnelURL string `json:"tunnel_url,omitempty"`
}
```

Add to `ServiceJSON`:

```go
type ServiceJSON struct {
	Port      int                  `json:"port"`
	EnvVar    string               `json:"env_var,omitempty"`
	Hostname  string               `json:"hostname,omitempty"`
	URL       string               `json:"url,omitempty"`
	Up        *bool                `json:"up,omitempty"`
	TunnelURL string               `json:"tunnel_url,omitempty"`
	Aliases   map[string]AliasJSON `json:"aliases,omitempty"`
}
```

In `buildStatus`, after the existing hostname/URL/tunnel assignment for a service, add alias population:

```go
			// Aliases
			if svcAliases, ok := alloc.Aliases[name]; ok && len(svcAliases) > 0 {
				aliasMap := make(map[string]AliasJSON, len(svcAliases))
				for aliasKey, aliasHostname := range svcAliases {
					aj := AliasJSON{
						Hostname: aliasHostname,
					}
					if u := urlutil.ServiceURL(aliasHostname, port, h.https); u != "" {
						aj.URL = u
					}
					if tunnelState != nil {
						if svcTunnels, ok := tunnelState[key]; ok {
							tunnelKey := name + "/alias/" + aliasKey
							if turl, ok := svcTunnels[tunnelKey]; ok {
								aj.TunnelURL = turl
							}
						}
					}
					aliasMap[aliasKey] = aj
				}
				sj.Aliases = aliasMap
			}
```

- [ ] **Step 2: Run full test suite to verify no regressions**

Run: `go test ./internal/daemon/ ./internal/dashboard/ -v`
Expected: PASS (dashboard tests should still pass — alias fields are omitempty).

- [ ] **Step 3: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat: expose aliases in dashboard API"
```

---

## Task 8: CLI — Display Aliases and Update Callers

**Files:**
- Modify: `cmd/render.go`
- Modify: `cmd/up.go`
- Modify: `cmd/envfiles.go`
- Modify: `cmd/share.go`

This task updates all CLI callers to pass aliases through the system and display them.

- [ ] **Step 1: Add aliases to svcJSON in render.go**

Add an `AliasJSON` type and `Aliases` field to `svcJSON`:

```go
type svcAliasJSON struct {
	Hostname string `json:"hostname"`
	URL      string `json:"url,omitempty"`
}

type svcJSON struct {
	Port          int                         `json:"port"`
	PreferredPort int                         `json:"preferred_port,omitempty"`
	EnvVar        string                      `json:"env_var"`
	Hostname      string                      `json:"hostname,omitempty"`
	URL           string                      `json:"url,omitempty"`
	EnvFiles      []string                    `json:"env_files"`
	Up            *bool                       `json:"up,omitempty"`
	Aliases       map[string]svcAliasJSON     `json:"aliases,omitempty"`
}
```

- [ ] **Step 2: Update buildServiceMap to include aliases**

Add an `aliases` parameter to `buildServiceMap`:

```go
func buildServiceMap(cfg *config.Config, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool) map[string]svcJSON {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		hostname := resolvedHostname(svc, hostnames, name)
		sj := svcJSON{
			Port:          ports[name],
			PreferredPort: svc.PreferredPort,
			EnvVar:        svc.EnvVar,
			Hostname:      hostname,
			URL:           urlutil.ServiceURL(hostname, ports[name], httpsEnabled),
			EnvFiles:      svc.EnvFiles,
		}
		if svcAliases, ok := aliases[name]; ok && len(svcAliases) > 0 {
			sj.Aliases = make(map[string]svcAliasJSON, len(svcAliases))
			for key, aliasHostname := range svcAliases {
				sj.Aliases[key] = svcAliasJSON{
					Hostname: aliasHostname,
					URL:      urlutil.ServiceURL(aliasHostname, ports[name], httpsEnabled),
				}
			}
		}
		services[name] = sj
	}
	return services
}
```

- [ ] **Step 3: Update styled output to show alias URLs**

In `render.go`, update `serviceURLSuffix` to return alias URLs too. Actually, it's cleaner to show all URLs (primary + aliases) as separate lines. Update `printServiceLineDetailed` and `printServiceLineCompact` to print alias URLs on subsequent lines:

Add a helper for alias URL lines:

```go
// printAliasURLs renders additional URL lines for a service's aliases, indented under the service.
func printAliasURLs(w io.Writer, aliases map[string]string, port int, httpsEnabled bool, indent string) {
	if len(aliases) == 0 {
		return
	}
	keys := slices.Sorted(maps.Keys(aliases))
	for _, key := range keys {
		aliasHostname := aliases[key]
		if u := urlutil.ServiceURL(aliasHostname, port, httpsEnabled); u != "" {
			lipgloss.Fprintln(w, indent+ui.UrlStyle.Render(u))
		}
	}
}
```

Update `printFlatServices` to accept and pass aliases:

```go
func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, portStatus map[int]bool, httpsEnabled bool) {
	for _, svcName := range serviceNames {
		printServiceLineDetailed(w, cfg, svcName, ports[svcName], hostnames, portStatus, httpsEnabled)
		if svcAliases, ok := aliases[svcName]; ok {
			printAliasURLs(w, svcAliases, ports[svcName], httpsEnabled, "    "+strings.Repeat(" ", 16+2+20+2+2+5))
		}
	}
}
```

Note: The exact indent width should match the existing line layout so alias URLs align under the primary URL. The precise value depends on the format string widths — measure and adjust during implementation.

- [ ] **Step 4: Update up.go to pass aliases through**

In `cmd/up.go`:

Update the hostname uniqueness check to also check aliases:

```go
	// Check hostname uniqueness across registry (primary + aliases)
	selfKey := registry.Key(cfg.Name, ctx.Instance)
	for svcName, hostname := range alloc.Hostnames {
		if conflictKey, found := reg.FindHostname(hostname, selfKey); found {
			return fmt.Errorf("hostname %q (service %q) conflicts with %s", hostname, svcName, conflictKey)
		}
	}
	for svcName, svcAliases := range alloc.Aliases {
		for aliasKey, hostname := range svcAliases {
			if conflictKey, found := reg.FindHostname(hostname, selfKey); found {
				return fmt.Errorf("alias %q hostname %q (service %q) conflicts with %s", aliasKey, hostname, svcName, conflictKey)
			}
		}
	}
```

Update `printUpJSON` and `printUpStyled` to pass aliases:

In `printUpJSON`, pass `alloc.Aliases` to `buildServiceMap`:

```go
Services: buildServiceMap(cfg, ports, alloc.Hostnames, alloc.Aliases, httpsEnabled),
```

In `printUpStyled`, pass aliases to `printFlatServices`:

```go
printFlatServices(w, cfg, serviceNames, ports, alloc.Hostnames, alloc.Aliases, nil, httpsEnabled)
```

- [ ] **Step 5: Update envfiles.go to pass aliases to ResolveComputed**

In `cmd/envfiles.go`, update `mergeEnvFiles` signature to accept aliases and pass them through:

```go
func mergeEnvFiles(dir string, cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool, tunnelURLs map[string]string) (map[string]map[string]string, error) {
```

Update the `ResolveComputed` call:

```go
resolvedComputed := allocation.ResolveComputed(cfg, instanceName, ports, hostnames, aliases, httpsEnabled, tunnelURLs)
```

Update `writeEnvFiles` to accept and pass aliases:

```go
func writeEnvFiles(
	dir string, cfg *config.Config, instanceName string,
	ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string,
	httpsEnabled bool, opts EnvWriteOptions,
) (*WriteResult, error) {
```

Pass aliases in the `mergeEnvFiles` call:

```go
resolvedComputed, err := mergeEnvFiles(dir, cfg, instanceName, ports, hostnames, aliases, httpsEnabled, opts.TunnelURLs)
```

- [ ] **Step 6: Update all callers of writeEnvFiles**

In `cmd/up.go`, pass `alloc.Aliases`:

```go
result, err := writeEnvFiles(dir, cfg, ctx.Instance, ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, EnvWriteOptions{...})
```

In `cmd/share.go`, pass aliases in both `writeEnvFiles` calls (the main call and the deferred cleanup call). Get aliases from the allocation:

```go
result, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, EnvWriteOptions{...})
```

And the deferred cleanup call:

```go
if _, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, alloc.Aliases, httpsEnabled, EnvWriteOptions{...}); err != nil {
```

- [ ] **Step 7: Update any other callers of buildServiceMap**

Search for `buildServiceMap(` and update all call sites to pass aliases. This includes `cmd/ports.go` and `cmd/status.go` if they exist.

Run: `grep -rn "buildServiceMap(" cmd/`

Pass `nil` for aliases at any call site where aliases aren't available (e.g., commands that don't load the allocation).

- [ ] **Step 8: Build and fix compile errors**

Run: `go build ./...`
Fix any remaining compile errors from changed function signatures.

- [ ] **Step 9: Run full test suite**

Run: `just test`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add cmd/render.go cmd/up.go cmd/envfiles.go cmd/share.go
git add -u cmd/  # catch any other modified cmd files
git commit -m "feat: display aliases in CLI output and pass through allocation"
```

---

## Task 9: Settings — max_tunnels

**Files:**
- Modify: `internal/settings/settings.go`
- Modify: `internal/settings/settings_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/settings/settings_test.go`:

```go
func TestLoadMaxTunnels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(`
[tunnels]
max = 12
`), 0644)

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Tunnels.Max != 12 {
		t.Errorf("max = %d, want 12", s.Tunnels.Max)
	}
}

func TestDefaultMaxTunnels(t *testing.T) {
	s := Defaults()
	if s.Tunnels.Max != 8 {
		t.Errorf("default max = %d, want 8", s.Tunnels.Max)
	}
}

func TestMaxTunnelsValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	os.WriteFile(path, []byte(`
[tunnels]
max = 0
`), 0644)

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for max=0")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/settings/ -run "TestLoadMaxTunnels|TestDefaultMaxTunnels|TestMaxTunnelsValidation" -v`
Expected: FAIL — no `Tunnels` field on `Settings`.

- [ ] **Step 3: Add Tunnels settings**

In `internal/settings/settings.go`, add:

```go
// TunnelSettings controls the behavior of the `outport share` tunnel feature.
type TunnelSettings struct {
	// Max is the maximum number of concurrent cloudflared tunnel processes.
	// When the cap is reached, primary hostnames are tunneled first, then
	// aliases in config order. Default: 8. Must be greater than 0.
	Max int
}
```

Add to `Settings`:

```go
type Settings struct {
	Dashboard DashboardSettings
	DNS       DNSSettings
	Tunnels   TunnelSettings
}
```

Update `Defaults`:

```go
func Defaults() Settings {
	return Settings{
		Dashboard: DashboardSettings{HealthInterval: 3 * time.Second},
		DNS:       DNSSettings{TTL: 60},
		Tunnels:   TunnelSettings{Max: 8},
	}
}
```

In `LoadFrom`, add parsing after the DNS section:

```go
	tunnels := cfg.Section("tunnels")
	if key, err := tunnels.GetKey("max"); err == nil {
		v, err := key.Int()
		if err != nil {
			return nil, fmt.Errorf("invalid tunnels max: %w", err)
		}
		s.Tunnels.Max = v
	}
```

In `validate`, add:

```go
	if s.Tunnels.Max <= 0 {
		return fmt.Errorf("tunnels max %d must be greater than 0", s.Tunnels.Max)
	}
```

Update `DefaultConfigContent` to include the new setting:

```go
func DefaultConfigContent() string {
	return `# Outport global settings
# Uncomment and change values to override defaults.
# Restart the daemon after changes: outport system restart

[dashboard]
# How often the dashboard checks whether services are accepting connections.
# Accepts Go duration syntax: 1s, 5s, 500ms. Minimum 1s.
# health_interval = 3s

[dns]
# Time-to-live in seconds for .test DNS responses. Lower values mean the
# browser picks up service changes faster, but increases DNS queries.
# ttl = 60

[tunnels]
# Maximum number of concurrent tunnel processes for outport share.
# Primary hostnames are tunneled first, then aliases.
# max = 8
`
}
```

- [ ] **Step 4: Run settings tests**

Run: `go test ./internal/settings/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/settings/settings.go internal/settings/settings_test.go
git commit -m "feat: add tunnels.max setting"
```

---

## Task 10: Share Command — Per-Hostname Tunnels Through Proxy

**Files:**
- Modify: `cmd/share.go`

This task rearchitects `outport share` to:
1. Build a per-hostname tunnel map (primary + aliases)
2. Route tunnels through the proxy (port 80) instead of directly to service ports
3. Register tunnel routes with HostOverride for Host header rewriting
4. Respect the `max_tunnels` cap

- [ ] **Step 1: Update share to build per-hostname tunnel map**

Replace the service→port map with a hostname→port map in `runShare`. The key change: instead of one tunnel per service, create one tunnel per hostname (primary + aliases), all pointing at port 80 (the proxy):

```go
func runShare(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("No ports allocated. Run 'outport up' first.")
	}

	services, err := resolveShareServices(ctx, args)
	if err != nil {
		return err
	}

	provider := cloudflare.New()
	if err := provider.CheckAvailable(); err != nil {
		return err
	}

	// Load max_tunnels setting
	settings, err := settingsPkg.Load()
	if err != nil {
		settings = &settingsPkg.Settings{Tunnels: settingsPkg.TunnelSettings{Max: 8}}
	}
	maxTunnels := settings.Tunnels.Max

	// Build ordered list of hostnames to tunnel: primaries first, then aliases
	type tunnelTarget struct {
		Label    string // "web" or "web/alias/app"
		Hostname string // "approvethis.test" or "app.approvethis.test"
	}
	var targets []tunnelTarget
	for _, name := range services {
		if hostname, ok := alloc.Hostnames[name]; ok {
			targets = append(targets, tunnelTarget{Label: name, Hostname: hostname})
		}
	}
	for _, name := range services {
		if svcAliases, ok := alloc.Aliases[name]; ok {
			aliasKeys := slices.Sorted(maps.Keys(svcAliases))
			for _, key := range aliasKeys {
				targets = append(targets, tunnelTarget{
					Label:    name + "/alias/" + key,
					Hostname: svcAliases[key],
				})
			}
		}
	}

	// Apply max_tunnels cap
	var skipped []string
	if len(targets) > maxTunnels {
		skipped = make([]string, 0, len(targets)-maxTunnels)
		for _, t := range targets[maxTunnels:] {
			skipped = append(skipped, t.Hostname)
		}
		targets = targets[:maxTunnels]
	}

	// All tunnels point to port 80 (the proxy)
	hostnamePorts := make(map[string]int, len(targets))
	for _, t := range targets {
		hostnamePorts[t.Label] = 80
	}

	mgr := tunnel.NewManager(provider, 15*time.Second)

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tunnels, err := mgr.StartAll(sigCtx, hostnamePorts)
	if err != nil {
		return fmt.Errorf("starting tunnels: %w", err)
	}

	// ... rest of share command adapted for new tunnel structure
```

This is a significant refactor of `share.go`. The full implementation should:
- Build `tunnelURLs` map for template vars using the label keys (`web`, `web/alias/app`)
- Write tunnel state for dashboard discovery
- Display output showing each hostname's tunnel URL
- Handle cleanup on exit

The exact code depends on the current shape of share.go and the tunnel state format. The implementor should follow the existing patterns in share.go and adapt them for the per-hostname model.

- [ ] **Step 2: Print skipped aliases warning**

After printing tunnel URLs, if any were skipped:

```go
	if len(skipped) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: tunnel limit reached (%d). Skipped: %s\n",
			maxTunnels, strings.Join(skipped, ", "))
	}
```

- [ ] **Step 3: Build and verify compilation**

Run: `go build ./...`
Expected: Compiles without errors.

- [ ] **Step 4: Run full test suite**

Run: `just test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/share.go
git commit -m "feat: per-hostname tunnels through proxy with Host rewriting"
```

---

## Task 11: Integration — Full Pipeline Test

- [ ] **Step 1: Run the full test suite**

Run: `just test`
Expected: All tests pass.

- [ ] **Step 2: Run the linter**

Run: `just lint`
Expected: Clean.

- [ ] **Step 3: Manual smoke test**

Create a test `outport.yml` with aliases and verify:
1. `outport up` registers all hostnames
2. `outport up --json` shows aliases in JSON output
3. `outport list` shows alias URLs
4. `outport down` cleans up
5. Running `outport up` again is idempotent

- [ ] **Step 4: Final commit for any remaining fixes**

```bash
git add -u
git commit -m "fix: integration fixes for hostname aliases"
```

---

## Task 12: Documentation

**Files:**
- Modify: `CLAUDE.md` (if architectural changes need noting)
- Modify: docs site files (config reference, commands)

- [ ] **Step 1: Update CLAUDE.md**

Add aliases to the template expansion section under Key Design Decisions:

Under "Template expansion", add `${service.alias.NAME}` and `${service.alias_url.NAME}` to the documented variables.

Under "`.test` hostnames", note that aliases register additional proxy routes to the same port.

- [ ] **Step 2: Update docs site config reference**

Add the `aliases` field to the service configuration documentation in the docs site.

- [ ] **Step 3: Update docs site commands reference**

Note alias display in `outport up`, `outport list`, `outport status`, and `outport share` output.

- [ ] **Step 4: Commit docs**

```bash
git add CLAUDE.md docs/
git commit -m "docs: document hostname aliases"
```

# Wildcard Subdomain Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `subdomains: true` to service definitions so all subdomains of a hostname route to the same port without explicit aliases.

**Architecture:** A `Subdomains bool` field flows from config through allocation into the registry. The daemon's `BuildRoutes` populates a separate `wildcards` map alongside the existing `routes` map. `RouteTable.Lookup` tries an exact match first, then strips the first DNS label and checks the wildcards map. TLS certs are already generated lazily per-hostname, so new subdomains just work.

**Tech Stack:** Go, Cobra CLI, miekg/dns, httputil.ReverseProxy

---

### Task 1: Add `Subdomains` field to config and validate

**Files:**
- Modify: `internal/config/config.go:191-231` (Service + rawService structs, validation)
- Modify: `internal/config/config.go:443-464` (mergeLocal)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestLoad_Subdomains(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
name: myproject
services:
  web:
    env_var: PORT
    hostname: myproject.test
    subdomains: true
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Services["web"].Subdomains {
		t.Error("expected Subdomains to be true")
	}
}

func TestLoad_SubdomainsDefaultsFalse(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
name: myproject
services:
  web:
    env_var: PORT
    hostname: myproject.test
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["web"].Subdomains {
		t.Error("expected Subdomains to default to false")
	}
}

func TestValidateSubdomainsRequiresHostname(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
name: myproject
services:
  web:
    env_var: PORT
    subdomains: true
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for subdomains without hostname")
	}
	if !strings.Contains(err.Error(), "subdomains") {
		t.Errorf("expected error about subdomains, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_Subdomains|TestValidateSubdomains" -v`
Expected: FAIL — `Subdomains` field does not exist.

- [ ] **Step 3: Add `Subdomains` field to Service and rawService structs**

In `internal/config/config.go`, add after the `Aliases` field (line 212):

```go
	// Subdomains enables wildcard subdomain routing for this service's primary
	// hostname. When true, all subdomains (e.g., *.myapp.test) route to the same
	// port. Requires Hostname to be set. Does not apply to aliases.
	Subdomains bool `yaml:"subdomains"`
```

In the `rawService` struct (line 231), add after `Aliases`:

```go
	Subdomains bool              `yaml:"subdomains"`
```

- [ ] **Step 4: Copy `Subdomains` in the normalize function**

Find the `normalize` function where `rawService` fields are copied to `Service`. Add:

```go
			svc.Subdomains = rawSvc.Subdomains
```

alongside the existing `svc.Aliases = rawSvc.Aliases` line.

- [ ] **Step 5: Add validation rule**

In `internal/config/config.go`, in the validation function, after the aliases-require-hostname check (line 576), add:

```go
		if svc.Subdomains && svc.Hostname == "" {
			return fmt.Errorf("service %q: subdomains requires a primary hostname", name)
		}
```

- [ ] **Step 6: Add mergeLocal support**

In `internal/config/config.go` in `mergeLocal` (around line 458), add after the `Aliases` merge block:

```go
		if localSvc.Subdomains {
			baseSvc.Subdomains = localSvc.Subdomains
		}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "TestLoad_Subdomains|TestValidateSubdomains" -v`
Expected: PASS

- [ ] **Step 8: Write the local override test**

Add to `internal/config/config_test.go`:

```go
func TestLoad_SubdomainsLocalOverride(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
name: myproject
services:
  web:
    env_var: PORT
    hostname: myproject.test
`)
	writeLocalConfig(t, dir, `
services:
  web:
    subdomains: true
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Services["web"].Subdomains {
		t.Error("expected Subdomains to be true from local override")
	}
}
```

- [ ] **Step 9: Run the local override test**

Run: `go test ./internal/config/ -run TestLoad_SubdomainsLocalOverride -v`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add subdomains field to service config with validation"
```

---

### Task 2: Add `Subdomains` to registry and allocation

**Files:**
- Modify: `internal/registry/registry.go:28-61` (Allocation struct)
- Modify: `internal/allocation/allocation.go:32-40` (Build function)
- Test: `internal/allocation/allocation_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/allocation/allocation_test.go`:

```go
func TestComputeSubdomains(t *testing.T) {
	cfg := &config.Config{
		Name: "myproject",
		Services: map[string]config.Service{
			"web": {Hostname: "myproject.test", Subdomains: true, EnvVar: "PORT"},
			"api": {Hostname: "api.myproject.test", EnvVar: "API_PORT"},
			"worker": {EnvVar: "WORKER_PORT"},
		},
	}

	result := allocation.ComputeSubdomains(cfg)

	if !result["web"] {
		t.Error("expected web to have subdomains=true")
	}
	if result["api"] {
		t.Error("expected api to not have subdomains")
	}
	if result["worker"] {
		t.Error("expected worker to not have subdomains")
	}
}

func TestComputeSubdomains_NoneSet(t *testing.T) {
	cfg := &config.Config{
		Name: "myproject",
		Services: map[string]config.Service{
			"web": {Hostname: "myproject.test", EnvVar: "PORT"},
		},
	}

	result := allocation.ComputeSubdomains(cfg)

	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/allocation/ -run TestComputeSubdomains -v`
Expected: FAIL — `ComputeSubdomains` does not exist.

- [ ] **Step 3: Add `Subdomains` field to Allocation struct**

In `internal/registry/registry.go`, add after the `Aliases` field (line 48):

```go
	// Subdomains maps service names to a boolean indicating wildcard subdomain
	// routing is enabled. When true for a service, all subdomains of its primary
	// hostname route to the same port (e.g., *.myapp.test → port).
	Subdomains map[string]bool `json:"subdomains,omitempty"`
```

- [ ] **Step 4: Implement `ComputeSubdomains`**

In `internal/allocation/allocation.go`, add after `ComputeAliases`:

```go
// ComputeSubdomains builds a map of service name to true for every service
// that has subdomains enabled. Services without subdomains are omitted.
func ComputeSubdomains(cfg *config.Config) map[string]bool {
	result := make(map[string]bool)
	for name, svc := range cfg.Services {
		if svc.Subdomains {
			result[name] = true
		}
	}
	return result
}
```

- [ ] **Step 5: Wire into Build function**

In `internal/allocation/allocation.go`, update the `Build` function (line 32) to include subdomains:

```go
func Build(cfg *config.Config, instanceName, dir string, ports map[string]int) registry.Allocation {
	return registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
		Hostnames:  ComputeHostnames(cfg, instanceName),
		Aliases:    ComputeAliases(cfg, instanceName),
		Subdomains: ComputeSubdomains(cfg),
		EnvVars:    computeEnvVars(cfg),
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/allocation/ -run TestComputeSubdomains -v`
Expected: PASS

- [ ] **Step 7: Run full test suite to check for regressions**

Run: `go test ./internal/allocation/ ./internal/registry/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/registry/registry.go internal/allocation/allocation.go internal/allocation/allocation_test.go
git commit -m "feat: add subdomains to registry allocation and allocation builder"
```

---

### Task 3: Add wildcard support to RouteTable and BuildRoutes

**Files:**
- Modify: `internal/daemon/routes.go:19-44` (RouteTable struct), `routes.go:50-55` (Lookup), `routes.go:57-66` (update), `routes.go:74-83` (UpdateWithAllocations), `routes.go:132-148` (BuildRoutes)
- Test: `internal/daemon/routes_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/daemon/routes_test.go`:

```go
func TestBuildRoutesWildcardSubdomains(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("realty120", "main", registry.Allocation{
		ProjectDir: "/src/realty120",
		Ports:      map[string]int{"web": 10384},
		Hostnames:  map[string]string{"web": "realty120.test"},
		Subdomains: map[string]bool{"web": true},
	})

	routes, wildcards := BuildRoutes(reg)

	if routes["realty120.test"].Port != 10384 {
		t.Errorf("exact route: got %d, want 10384", routes["realty120.test"].Port)
	}
	if wildcards["realty120.test"].Port != 10384 {
		t.Errorf("wildcard: got %d, want 10384", wildcards["realty120.test"].Port)
	}
}

func TestBuildRoutesNoWildcardWithoutFlag(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("myapp", "main", registry.Allocation{
		ProjectDir: "/src/myapp",
		Ports:      map[string]int{"web": 24920},
		Hostnames:  map[string]string{"web": "myapp.test"},
	})

	_, wildcards := BuildRoutes(reg)

	if _, ok := wildcards["myapp.test"]; ok {
		t.Error("expected no wildcard without subdomains flag")
	}
}

func TestLookupWildcardSubdomain(t *testing.T) {
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{"realty120.test": {Port: 10384}},
		map[string]route{"realty120.test": {Port: 10384}},
	)

	tests := []struct {
		name     string
		hostname string
		wantPort int
		wantOK   bool
	}{
		{"exact match", "realty120.test", 10384, true},
		{"subdomain match", "rp.realty120.test", 10384, true},
		{"deep subdomain match", "foo.rp.realty120.test", 0, false},
		{"no match", "other.test", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := rt.Lookup(tt.hostname)
			if ok != tt.wantOK {
				t.Errorf("Lookup(%q) ok = %v, want %v", tt.hostname, ok, tt.wantOK)
			}
			if r.Port != tt.wantPort {
				t.Errorf("Lookup(%q) port = %d, want %d", tt.hostname, r.Port, tt.wantPort)
			}
		})
	}
}

func TestBuildRoutesWildcardWithInstanceSuffix(t *testing.T) {
	reg := &registry.Registry{Projects: make(map[string]registry.Allocation)}
	reg.Set("realty120", "bkrm", registry.Allocation{
		ProjectDir: "/src/realty120-worktree",
		Ports:      map[string]int{"web": 10384},
		Hostnames:  map[string]string{"web": "realty120-bkrm.test"},
		Subdomains: map[string]bool{"web": true},
	})

	routes, wildcards := BuildRoutes(reg)

	if routes["realty120-bkrm.test"].Port != 10384 {
		t.Errorf("exact route: got %d, want 10384", routes["realty120-bkrm.test"].Port)
	}
	if wildcards["realty120-bkrm.test"].Port != 10384 {
		t.Errorf("wildcard: got %d, want 10384", wildcards["realty120-bkrm.test"].Port)
	}
}

func TestLookupExactWinsOverWildcard(t *testing.T) {
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{
			"realty120.test":     {Port: 10384},
			"api.realty120.test": {Port: 20000},
		},
		map[string]route{"realty120.test": {Port: 10384}},
	)

	// Exact match should win
	r, ok := rt.Lookup("api.realty120.test")
	if !ok {
		t.Fatal("expected match for api.realty120.test")
	}
	if r.Port != 20000 {
		t.Errorf("exact match: got port %d, want 20000", r.Port)
	}

	// Wildcard should catch other subdomains
	r, ok = rt.Lookup("rp.realty120.test")
	if !ok {
		t.Fatal("expected wildcard match for rp.realty120.test")
	}
	if r.Port != 10384 {
		t.Errorf("wildcard match: got port %d, want 10384", r.Port)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run "TestBuildRoutesWildcard|TestBuildRoutesNoWildcard|TestLookupWildcard|TestLookupExactWins" -v`
Expected: FAIL — `BuildRoutes` returns one value, `updateWithWildcards` does not exist.

- [ ] **Step 3: Add `wildcards` field to RouteTable**

In `internal/daemon/routes.go`, add to the `RouteTable` struct (after line 37):

```go
	wildcards   map[string]route               // parent hostname -> route (for subdomains: true)
```

- [ ] **Step 4: Update `Lookup` with wildcard fallback**

Replace the `Lookup` method (lines 50-55):

```go
func (rt *RouteTable) Lookup(hostname string) (route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if r, ok := rt.routes[hostname]; ok {
		return r, true
	}
	if idx := strings.Index(hostname, "."); idx > 0 {
		parent := hostname[idx+1:]
		if r, ok := rt.wildcards[parent]; ok {
			return r, true
		}
	}
	return route{}, false
}
```

Add `"strings"` to the import block if not already present.

- [ ] **Step 5: Add `updateWithWildcards` test helper and update `update`**

Add a new method for tests:

```go
// updateWithWildcards swaps both routing maps atomically. Used in tests.
func (rt *RouteTable) updateWithWildcards(routes, wildcards map[string]route) {
	rt.mu.Lock()
	rt.routes = routes
	rt.wildcards = wildcards
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}
```

Update the existing `update` method to also clear wildcards:

```go
func (rt *RouteTable) update(routes map[string]route) {
	rt.mu.Lock()
	rt.routes = routes
	rt.wildcards = nil
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}
```

- [ ] **Step 6: Update `UpdateWithAllocations` to build wildcards**

Replace `UpdateWithAllocations` (lines 74-83):

```go
func (rt *RouteTable) UpdateWithAllocations(routes map[string]route, wildcards map[string]route, allocs map[string]registry.Allocation) {
	rt.mu.Lock()
	rt.routes = routes
	rt.wildcards = wildcards
	rt.allocations = allocs
	rt.ports = deduplicatePorts(allocs)
	rt.mu.Unlock()
	if rt.OnUpdate != nil {
		rt.OnUpdate()
	}
}
```

- [ ] **Step 7: Update `BuildRoutes` to return wildcards**

Replace `BuildRoutes` (lines 132-148):

```go
func BuildRoutes(reg *registry.Registry) (map[string]route, map[string]route) {
	routes := make(map[string]route)
	wildcards := make(map[string]route)
	for _, alloc := range reg.Projects {
		if alloc.Hostnames == nil {
			continue
		}
		for svcName, hostname := range alloc.Hostnames {
			routes[hostname] = route{Port: alloc.Ports[svcName]}
			if alloc.Subdomains[svcName] {
				wildcards[hostname] = route{Port: alloc.Ports[svcName]}
			}
		}
		for svcName, svcAliases := range alloc.Aliases {
			for _, aliasHostname := range svcAliases {
				routes[aliasHostname] = route{Port: alloc.Ports[svcName]}
			}
		}
	}
	return routes, wildcards
}
```

- [ ] **Step 8: Update callers of `BuildRoutes` and `UpdateWithAllocations`**

In `internal/daemon/routes.go`, update `rebuildFromFile` (line 303):

```go
	routes, wildcards := BuildRoutes(&reg)
	rt.UpdateWithAllocations(routes, wildcards, reg.Projects)
```

Update `MergeTunnelRoutes` to preserve wildcards (no change needed — it only adds to `routes`).

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run "TestBuildRoutesWildcard|TestBuildRoutesNoWildcard|TestLookupWildcard|TestLookupExactWins" -v`
Expected: PASS

- [ ] **Step 10: Fix any existing tests broken by signature changes**

Run: `go test ./internal/daemon/ -v`

The existing `TestBuildRoutes*` tests call `BuildRoutes(reg)` and expect one return value. Update them to capture both:

```go
routes, _ := BuildRoutes(reg)
```

Fix all instances in `internal/daemon/routes_test.go`.

- [ ] **Step 11: Run full daemon test suite**

Run: `go test ./internal/daemon/ -v`
Expected: PASS

- [ ] **Step 12: Commit**

```bash
git add internal/daemon/routes.go internal/daemon/routes_test.go
git commit -m "feat: add wildcard subdomain lookup to RouteTable and BuildRoutes"
```

---

### Task 4: Add proxy integration test for wildcard routing

**Files:**
- Test: `internal/daemon/proxy_test.go`

- [ ] **Step 1: Write the proxy integration test**

Add to `internal/daemon/proxy_test.go`:

```go
func TestProxyRoutesWildcardSubdomain(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tenant backend"))
	}))
	defer backend.Close()

	port := backendPort(t, backend)
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{"realty120.test": {Port: port}},
		map[string]route{"realty120.test": {Port: port}},
	)

	proxy := NewProxy(rt)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	// Subdomain request should route to the same backend
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "rp.realty120.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "tenant backend" {
		t.Errorf("got %q, want %q", body, "tenant backend")
	}
}

func TestProxyExactMatchWinsOverWildcard(t *testing.T) {
	wildcard := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("wildcard"))
	}))
	defer wildcard.Close()

	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("exact"))
	}))
	defer exact.Close()

	wildcardPort := backendPort(t, wildcard)
	exactPort := backendPort(t, exact)
	rt := &RouteTable{}
	rt.updateWithWildcards(
		map[string]route{
			"realty120.test":     {Port: wildcardPort},
			"api.realty120.test": {Port: exactPort},
		},
		map[string]route{"realty120.test": {Port: wildcardPort}},
	)

	proxy := NewProxy(rt)
	srv := httptest.NewServer(proxy)
	defer srv.Close()

	// Exact match should hit the exact backend
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.Host = "api.realty120.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "exact" {
		t.Errorf("got %q, want %q", body, "exact")
	}
}
```

- [ ] **Step 2: Run proxy tests**

Run: `go test ./internal/daemon/ -run "TestProxyRoutesWildcard|TestProxyExactMatch" -v`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS — all packages compile and pass.

- [ ] **Step 4: Commit**

```bash
git add internal/daemon/proxy_test.go
git commit -m "test: add proxy integration tests for wildcard subdomain routing"
```

---

### Task 5: Update CLI output to show "(+ subdomains)" indicator

**Files:**
- Modify: `cmd/render.go:22-31` (svcJSON struct), `cmd/render.go:60-84` (buildServiceMap), `cmd/render.go:192-205` (serviceURLSuffix)
- Modify: `cmd/render.go:167-174` (printFlatServices signature)
- Modify: `cmd/up.go:215` (printUpStyled call)

- [ ] **Step 1: Add `Subdomains` to svcJSON**

In `cmd/render.go`, add to the `svcJSON` struct (after the `Aliases` field, line 30):

```go
	Subdomains bool                     `json:"subdomains,omitempty"`
```

- [ ] **Step 2: Populate `Subdomains` in buildServiceMap**

In `cmd/render.go`, inside `buildServiceMap` (after the aliases block, around line 81), add:

```go
		if cfg.Services[name].Subdomains {
			sj.Subdomains = true
		}
```

- [ ] **Step 3: Add "(+ subdomains)" to styled URL suffix**

In `cmd/render.go`, update `serviceURLSuffix` (lines 192-205). After the URL is built, append a subdomains indicator:

```go
func serviceURLSuffix(cfg *config.Config, svcName string, hostnames map[string]string, port int, httpsEnabled bool) string {
	svc, ok := cfg.Services[svcName]
	if !ok {
		return ""
	}
	hostname := resolvedHostname(svc, hostnames, svcName)
	if u := urlutil.ServiceURL(hostname, port, httpsEnabled); u != "" {
		suffix := "  " + ui.UrlStyle.Render(u)
		if svc.Subdomains {
			suffix += "  " + ui.DimStyle.Render("(+ subdomains)")
		}
		return suffix
	}
	if hostname != "" {
		return "  " + ui.HostnameStyle.Render(hostname)
	}
	return ""
}
```

- [ ] **Step 4: Run the build to verify compilation**

Run: `go build ./...`
Expected: compiles successfully.

- [ ] **Step 5: Commit**

```bash
git add cmd/render.go
git commit -m "feat: show (+ subdomains) indicator in CLI output and JSON"
```

---

### Task 6: Update dashboard to show subdomains indicator

**Files:**
- Modify: `internal/dashboard/handler.go:85-114` (ServiceJSON struct)
- Modify: `internal/dashboard/handler.go:420-460` (service JSON building)

- [ ] **Step 1: Add `Subdomains` to ServiceJSON**

In `internal/dashboard/handler.go`, add to the `ServiceJSON` struct (after the `Aliases` field, line 113):

```go
	// Subdomains indicates whether wildcard subdomain routing is enabled for
	// this service. When true, all subdomains of the primary hostname route to
	// the same port.
	Subdomains bool `json:"subdomains,omitempty"`
```

- [ ] **Step 2: Populate `Subdomains` from allocation data**

In `internal/dashboard/handler.go`, in the service-building loop (after the aliases block, around line 470), add:

```go
			if alloc.Subdomains[name] {
				sj.Subdomains = true
			}
```

- [ ] **Step 3: Run the build**

Run: `go build ./...`
Expected: compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat: expose subdomains flag in dashboard API"
```

---

### Task 7: Add tunnel hint for subdomain services in `outport share`

**Files:**
- Modify: `cmd/share.go:76-112` (tunnel target building section)

- [ ] **Step 1: Add subdomain hint after tunnel output**

In `cmd/share.go`, find where tunnel output is printed. After the existing sharing output, add a check for services with subdomains. Find the `printShareStyled` function and the section where skipped targets are warned about. After that section, add a note for subdomain services.

Locate the section after tunnels are started and output is printed. Add the following logic after the existing output (around where `skippedTargets` warning is printed):

```go
		// Print note for services with subdomain routing
		for _, svcName := range services {
			svc := ctx.Cfg.Services[svcName]
			if svc.Subdomains {
				hostname := alloc.Hostnames[svcName]
				fmt.Fprintf(cmd.OutOrStderr(), "\nNote: subdomain routing for %s is local-only (tunnels use explicit hostnames)\n", hostname)
			}
		}
```

- [ ] **Step 2: Run the build**

Run: `go build ./...`
Expected: compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add cmd/share.go
git commit -m "feat: print subdomain routing note during outport share"
```

---

### Task 8: Update docs and finalize

**Files:**
- Modify: `docs/reference/configuration.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add `subdomains` to configuration reference**

In `docs/reference/configuration.md`, find the service fields section and add documentation for the `subdomains` field. Add it after the `aliases` documentation:

```markdown
### `subdomains`

**Type:** `bool` (default: `false`)

Enables wildcard subdomain routing for this service's primary hostname. When `true`, all subdomains of the hostname (e.g., `*.myapp.test`) route to the same port — no explicit aliases needed.

This is useful for multi-tenant apps that use subdomains to identify tenants:

```yaml
services:
  web:
    env_var: PORT
    hostname: realty120.test
    subdomains: true
```

With this config, `rp.realty120.test`, `tina-snow.realty120.test`, and any other subdomain all route to the same port as `realty120.test`.

**Rules:**
- Requires a primary `hostname`
- Applies to the primary hostname only, not aliases
- Exact hostname matches (from other services or aliases) take precedence over the wildcard
- Subdomain routing is local-only — `outport share` tunnels explicit hostnames, not wildcards

**Combining with explicit aliases:**

```yaml
services:
  web:
    hostname: realty120.test
    subdomains: true           # *.realty120.test → this port

  api:
    hostname: api.realty120.test   # exact match wins → different port
```
```

- [ ] **Step 2: Update CLAUDE.md**

Add `subdomains: true` to the hostname-related design decisions section. Find the "`.test` hostnames" bullet and add after the aliases sentence:

```
Services can also enable `subdomains: true` for wildcard subdomain routing — all subdomains of the primary hostname route to the same port. Exact matches (other services or aliases) take precedence over wildcards.
```

- [ ] **Step 3: Run full test suite**

Run: `just test`
Expected: PASS

- [ ] **Step 4: Run lint**

Run: `just lint`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add docs/reference/configuration.md CLAUDE.md
git commit -m "docs: add subdomains configuration reference and update architecture notes"
```

- [ ] **Step 6: Run finalize checklist**

Verify:
- `just lint` passes
- `just test` passes
- `--json` output includes `subdomains` field for changed commands
- CLAUDE.md reflects the new feature
- Docs site updated with new config field

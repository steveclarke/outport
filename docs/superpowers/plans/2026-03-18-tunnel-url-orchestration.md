# Tunnel URL Orchestration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `outport share` tunnels HTTP services, rewrite `.env` files with tunnel URLs so computed values (CORS, API URLs) resolve to public tunnel URLs automatically — and revert on exit.

**Architecture:** Extract the env-writing pipeline (`buildTemplateVars` → `resolveComputedFromAlloc` → `mergeEnvFiles`) to accept an optional tunnel URL override map. `outport share` calls it after tunnels start (with overrides) and again on exit (without overrides). No new config fields or template variables.

**Tech Stack:** Go, Cobra CLI, existing internal packages (config, dotenv, tunnel)

**Spec:** `docs/superpowers/specs/2026-03-18-tunnel-url-orchestration-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/up.go` | Modify | `buildTemplateVars` and `resolveComputedFromAlloc` gain `tunnelURLs` parameter |
| `cmd/rename.go` | Modify | `mergeEnvFiles` gains `tunnelURLs` parameter, passes through |
| `cmd/promote.go` | Modify | Two `mergeEnvFiles` call sites gain `nil` argument |
| `cmd/share.go` | Modify | After tunnels start, rewrite `.env`; on exit, revert `.env` |
| `cmd/cmd_test.go` | Modify | Tests for tunnel URL override in template vars and share env rewrite |

---

## Chunk 1: Pipeline Enhancement

### Task 1: Add tunnelURLs parameter to buildTemplateVars

**Files:**
- Modify: `cmd/up.go:226-254` (buildTemplateVars function)
- Modify: `cmd/up.go:258-263` (resolveComputedFromAlloc — passes through)
- Test: `cmd/cmd_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/cmd_test.go`:

```go
func TestBuildTemplateVars_TunnelOverrides(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {
				EnvVar:   "RAILS_PORT",
				Protocol: "http",
				Hostname: "myapp.test",
			},
			"postgres": {
				EnvVar: "DB_PORT",
			},
		},
	}
	ports := map[string]int{"rails": 3000, "postgres": 5432}
	hostnames := map[string]string{"rails": "myapp.test"}
	tunnelURLs := map[string]string{"rails": "https://abc-def.trycloudflare.com"}

	vars := buildTemplateVars(cfg, "main", ports, hostnames, true, tunnelURLs)

	// Tunneled service: url overridden, url:direct stays localhost
	if got := vars["rails.url"]; got != "https://abc-def.trycloudflare.com" {
		t.Errorf("rails.url = %q, want tunnel URL", got)
	}
	if got := vars["rails.url:direct"]; got != "http://localhost:3000" {
		t.Errorf("rails.url:direct = %q, want localhost", got)
	}

	// Non-tunneled service: unchanged
	if got := vars["postgres.port"]; got != "5432" {
		t.Errorf("postgres.port = %q, want 5432", got)
	}

	// Hostname stays the same (only url changes)
	if got := vars["rails.hostname"]; got != "myapp.test" {
		t.Errorf("rails.hostname = %q, want myapp.test", got)
	}
}

func TestBuildTemplateVars_NilTunnelURLs(t *testing.T) {
	cfg := &config.Config{
		Name: "myapp",
		Services: map[string]config.Service{
			"rails": {
				EnvVar:   "RAILS_PORT",
				Protocol: "http",
				Hostname: "myapp.test",
			},
		},
	}
	ports := map[string]int{"rails": 3000}
	hostnames := map[string]string{"rails": "myapp.test"}

	vars := buildTemplateVars(cfg, "main", ports, hostnames, true, nil)

	if got := vars["rails.url"]; got != "https://myapp.test" {
		t.Errorf("rails.url = %q, want https://myapp.test", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestBuildTemplateVars_Tunnel -v`
Expected: FAIL — `buildTemplateVars` doesn't accept a 6th argument yet.

- [ ] **Step 3: Update buildTemplateVars to accept tunnelURLs**

In `cmd/up.go`, change the signature and add the override logic:

```go
// buildTemplateVars builds the template variable map from services and allocated ports.
// When tunnelURLs is non-nil, ${service.url} resolves to the tunnel URL for tunneled services.
// ${service.url:direct} always resolves to localhost (unaffected by tunnels).
func buildTemplateVars(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]string {
	vars := make(map[string]string)
	if instanceName == "main" {
		vars["instance"] = ""
	} else {
		vars["instance"] = instanceName
	}
	for name, svc := range cfg.Services {
		portStr := fmt.Sprintf("%d", ports[name])
		vars[name+".port"] = portStr

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h
			protocol := svc.Protocol
			if protocol == "" {
				protocol = "http"
			}

			// Tunnel URL override for browser-facing URL
			if tunnelURL, hasTunnel := tunnelURLs[name]; hasTunnel {
				vars[name+".url"] = tunnelURL
			} else {
				vars[name+".url"] = fmt.Sprintf("%s://%s", effectiveScheme(protocol, h, httpsEnabled), h)
			}
			vars[name+".url:direct"] = fmt.Sprintf("http://localhost:%s", portStr)
		} else {
			hostname := svc.Hostname
			if hostname == "" {
				hostname = "localhost"
			}
			vars[name+".hostname"] = hostname
		}
	}
	return vars
}
```

- [ ] **Step 4: Update resolveComputedFromAlloc to pass through tunnelURLs**

In `cmd/up.go`, update the function:

```go
func resolveComputedFromAlloc(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]map[string]string {
	if len(cfg.Computed) == 0 {
		return nil
	}
	templateVars := buildTemplateVars(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
	return config.ResolveComputed(cfg.Computed, templateVars)
}
```

- [ ] **Step 5: Update mergeEnvFiles signature and all callers**

In `cmd/rename.go`, update `mergeEnvFiles` signature to accept and pass through `tunnelURLs`:

```go
func mergeEnvFiles(dir string, cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) (map[string]map[string]string, error) {
```

And update the call inside it:

```go
resolvedComputed := resolveComputedFromAlloc(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
```

Update all existing call sites to pass `nil` (no behavior change):

- `cmd/up.go:122` `runUp`: `mergeEnvFiles(dir, cfg, ctx.Instance, ports, alloc.Hostnames, httpsEnabled, nil)`
- `cmd/rename.go:66` `runRename`: `mergeEnvFiles(ctx.Dir, cfg, newName, oldAlloc.Ports, newAlloc.Hostnames, httpsEnabled, nil)`
- `cmd/promote.go:65` `runPromote` (demoted instance): `mergeEnvFiles(mainAlloc.ProjectDir, cfg, demotedTo, mainAlloc.Ports, demotedAlloc.Hostnames, httpsEnabled, nil)`
- `cmd/promote.go:77` `runPromote` (promoted instance): `mergeEnvFiles(ctx.Dir, cfg, "main", currentAlloc.Ports, promotedAlloc.Hostnames, httpsEnabled, nil)`

- [ ] **Step 6: Run tests to verify everything passes**

Run: `go test ./cmd/ -v`
Expected: ALL PASS including new tunnel override tests.

- [ ] **Step 7: Run lint**

Run: `golangci-lint run`
Expected: 0 issues.

- [ ] **Step 8: Commit**

```bash
git add cmd/up.go cmd/rename.go cmd/promote.go cmd/cmd_test.go
git commit -m "feat: add tunnelURLs parameter to env-writing pipeline

buildTemplateVars accepts optional tunnel URL overrides.
When set, \${service.url} resolves to the tunnel URL instead
of the .test hostname. \${service.url:direct} stays localhost.
All existing callers pass nil (no behavior change)."
```

---

### Task 2: Wire share command to rewrite .env with tunnel URLs

**Files:**
- Modify: `cmd/share.go:31-85` (runShare function)
- Test: `cmd/cmd_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/cmd_test.go`:

```go
const testConfigWithComputedAndHostnames = `name: testapp
services:
  rails:
    preferred_port: 3000
    env_var: RAILS_PORT
    protocol: http
    hostname: testapp.test
  vite:
    preferred_port: 5173
    env_var: VITE_PORT
    protocol: http
    hostname: testapp-vite.test
  postgres:
    preferred_port: 5432
    env_var: DATABASE_PORT
computed:
  API_URL:
    value: "${rails.url}/api"
    env_file: .env
  API_URL_DIRECT:
    value: "${rails.url:direct}/api"
    env_file: .env
  CORS_ORIGINS:
    value: "${vite.url}"
    env_file: .env
`

func TestMergeEnvFiles_WithTunnelURLs(t *testing.T) {
	dir := setupProject(t, testConfigWithComputedAndHostnames)

	ports := map[string]int{"rails": 3000, "vite": 5173, "postgres": 5432}
	hostnames := map[string]string{"rails": "testapp.test", "vite": "testapp-vite.test"}
	tunnelURLs := map[string]string{
		"rails": "https://abc.trycloudflare.com",
		"vite":  "https://def.trycloudflare.com",
	}

	// Write with tunnel URLs
	_, err := mergeEnvFiles(dir, mustLoadConfig(t, dir), "main", ports, hostnames, false, tunnelURLs)
	if err != nil {
		t.Fatal(err)
	}

	env := readEnvFile(t, filepath.Join(dir, ".env"))

	// Computed values using ${service.url} should have tunnel URLs
	if got := env["API_URL"]; got != "https://abc.trycloudflare.com/api" {
		t.Errorf("API_URL = %q, want tunnel-based URL", got)
	}
	if got := env["CORS_ORIGINS"]; got != "https://def.trycloudflare.com" {
		t.Errorf("CORS_ORIGINS = %q, want tunnel-based URL", got)
	}

	// Computed values using ${service.url:direct} should stay localhost
	if got := env["API_URL_DIRECT"]; got != "http://localhost:3000/api" {
		t.Errorf("API_URL_DIRECT = %q, want localhost URL", got)
	}

	// Service port env vars should still be present
	if got := env["RAILS_PORT"]; got != "3000" {
		t.Errorf("RAILS_PORT = %q, want 3000", got)
	}

	// Now revert (nil tunnelURLs) — simulates cleanup on exit
	_, err = mergeEnvFiles(dir, mustLoadConfig(t, dir), "main", ports, hostnames, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	env = readEnvFile(t, filepath.Join(dir, ".env"))

	// Should be back to local URLs
	if got := env["API_URL"]; got != "http://testapp.test/api" {
		t.Errorf("after revert: API_URL = %q, want local URL", got)
	}
	if got := env["CORS_ORIGINS"]; got != "http://testapp-vite.test" {
		t.Errorf("after revert: CORS_ORIGINS = %q, want local URL", got)
	}
}
```

Add these test helpers if they don't exist:

```go
func mustLoadConfig(t *testing.T, dir string) *config.Config {
	t.Helper()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
```

- [ ] **Step 2: Run test to verify the pipeline works end to end**

Run: `go test ./cmd/ -run TestMergeEnvFiles_WithTunnelURLs -v`
Expected: PASS — the pipeline changes from Task 1 already support tunnel URL overrides through `mergeEnvFiles`. This test validates the full flow (template vars → computed values → .env file) before wiring it into the share command.

- [ ] **Step 3: Update runShare to rewrite .env files**

In `cmd/share.go`, update `runShare` after tunnels start:

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

	mgr := tunnel.NewManager(provider, 15*time.Second)

	// Build service→port map
	svcPorts := make(map[string]int)
	for _, name := range services {
		svcPorts[name] = alloc.Ports[name]
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tunnels, err := mgr.StartAll(sigCtx, svcPorts)
	if err != nil {
		return fmt.Errorf("starting tunnels: %w", err)
	}
	defer func() {
		mgr.StopAll()
		// Revert .env files to local URLs (best-effort; user can run 'outport up' if this fails)
		revertHTTPS := certmanager.IsCAInstalled()
		_, _ = mergeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, revertHTTPS, nil)
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), ui.SuccessStyle.Render("Restored .env files to local URLs."))
		fmt.Fprintln(cmd.OutOrStdout(), ui.DimStyle.Render("Restart your services to revert to local development."))
	}()

	// Sort once for deterministic output in both modes
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Service < tunnels[j].Service
	})

	// Build tunnel URL map and rewrite .env files
	tunnelURLs := make(map[string]string)
	for _, tun := range tunnels {
		tunnelURLs[tun.Service] = tun.URL
	}

	httpsEnabled := certmanager.IsCAInstalled()
	if _, err := mergeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, tunnelURLs); err != nil {
		return fmt.Errorf("writing tunnel URLs to .env: %w", err)
	}

	if jsonFlag {
		if err := printShareJSON(cmd, tunnels); err != nil {
			return err
		}
	} else {
		printShareStyled(cmd, tunnels)
	}

	// Block until signal
	<-sigCtx.Done()
	return nil
}
```

Add import for `certmanager` at the top of `cmd/share.go`:

```go
import (
	"context"
	"fmt"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/tunnel"
	"github.com/outport-app/outport/internal/tunnel/cloudflare"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 4: Update printShareStyled with .env messaging**

```go
func printShareStyled(cmd *cobra.Command, tunnels []*tunnel.Tunnel) {
	w := cmd.OutOrStdout()

	lipgloss.Fprintln(w, fmt.Sprintf("Sharing %d %s:",
		len(tunnels), pluralize(len(tunnels), "service", "services")))
	lipgloss.Fprintln(w)

	for _, tun := range tunnels {
		line := fmt.Sprintf("  %s  %s %s localhost:%d",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", tun.Service)),
			ui.UrlStyle.Render(tun.URL),
			ui.Arrow,
			tun.Port,
		)
		lipgloss.Fprintln(w, line)
	}

	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.SuccessStyle.Render("Updated .env files with tunnel URLs."))
	lipgloss.Fprintln(w, ui.DimStyle.Render("Restart your services to pick up the new URLs."))
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("Press Ctrl+C to stop sharing."))
}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./cmd/ -v`
Expected: ALL PASS.

- [ ] **Step 6: Run lint**

Run: `golangci-lint run`
Expected: 0 issues.

- [ ] **Step 7: Commit**

```bash
git add cmd/share.go cmd/cmd_test.go
git commit -m "feat: outport share rewrites .env with tunnel URLs

After tunnels start, rewrites .env files so \${service.url} computed
values resolve to tunnel URLs. On exit, reverts to local URLs.
Prints messages telling the user to restart services.

Closes #17"
```

---

## Chunk 2: JSON Output and Documentation

### Task 3: Enhance --json output for share

**Files:**
- Modify: `cmd/share.go` (shareJSON struct and printShareJSON)
- Test: `cmd/cmd_test.go`

- [ ] **Step 1: Update shareJSON struct**

In `cmd/share.go`, extend the JSON output to include computed values. Note: `computedJSON` and `buildComputedMap` are already defined in `cmd/up.go` and accessible within the `cmd` package:

```go
type shareJSON struct {
	Tunnels  []tunnelJSON            `json:"tunnels"`
	Computed map[string]computedJSON `json:"computed,omitempty"`
}
```

- [ ] **Step 2: Update printShareJSON to accept computed values**

```go
func printShareJSON(cmd *cobra.Command, tunnels []*tunnel.Tunnel, cfg *config.Config, resolvedComputed map[string]map[string]string) error {
	out := shareJSON{}
	for _, tun := range tunnels {
		out.Tunnels = append(out.Tunnels, tunnelJSON{
			Service: tun.Service,
			URL:     tun.URL,
			Port:    tun.Port,
		})
	}
	out.Computed = buildComputedMap(cfg.Computed, resolvedComputed)
	return writeJSON(cmd, out)
}
```

- [ ] **Step 3: Update runShare to pass computed values to printShareJSON**

Capture the return value from `mergeEnvFiles` and pass it to `printShareJSON`:

```go
resolvedComputed, err := mergeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, tunnelURLs)
if err != nil {
	return fmt.Errorf("writing tunnel URLs to .env: %w", err)
}

if jsonFlag {
	if err := printShareJSON(cmd, tunnels, ctx.Cfg, resolvedComputed); err != nil {
		return err
	}
} else {
	printShareStyled(cmd, tunnels)
}
```

- [ ] **Step 4: Run tests and lint**

Run: `go test ./cmd/ -v && golangci-lint run`
Expected: ALL PASS, 0 lint issues.

- [ ] **Step 5: Commit**

```bash
git add cmd/share.go
git commit -m "feat: include computed values in share --json output"
```

---

### Task 4: Update documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `docs/guide/tips.md`
- Modify: `docs/reference/commands.md`
- Modify: `skills/outport/SKILL.md`

- [ ] **Step 1: Update docs**

Use the `/update-docs` skill to audit all documentation locations and update any that reference `outport share` to mention the tunnel URL orchestration behavior:

- `outport share` now rewrites `.env` files with tunnel URLs
- Computed values using `${service.url}` automatically resolve to tunnel URLs
- On exit, `.env` files revert to local URLs
- Users should restart services after `outport share` starts and after it stops

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "docs: document tunnel URL orchestration in outport share"
```

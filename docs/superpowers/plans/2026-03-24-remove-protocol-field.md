# Remove Protocol Field — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `protocol` config field entirely. Hostname presence alone determines whether a service is web-routable.

**Architecture:** The `protocol` field on services is removed from config, allocation, registry, and all commands. Any code that checked `protocol == "http" || protocol == "https"` now checks `hostname != ""` (from config) or hostname presence in `alloc.Hostnames` (from registry). URL construction assumes `http` as the protocol for all hostname-bearing services. The `${service.protocol}` template variable is removed. The `Protocols` map is removed from `registry.Allocation`. No backward compatibility — this is a clean break.

**Tech Stack:** Go, Cobra CLI, golangci-lint

**Key design rule:** A service with a `hostname` is a web service (HTTP, proxied, openable, shareable, QR-codeable). A service without a `hostname` is an infrastructure service (port only).

**Compilation note:** Removing the `Protocol` field from the config struct breaks all packages that reference it. Tasks 1-7 must be applied together before `go build` will succeed. Each task is a logical unit for review, but they form one atomic change. Commit once at the end after all Go changes compile and pass.

---

### Task 1: Config — Remove Protocol from struct and validation

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Update the Service and rawService structs**

In `config.go`, remove `Protocol string` from both the `Service` struct (~line 131) and `rawService` struct (~line 141).

- [ ] **Step 2: Remove "protocol" from validFields map**

Remove `"protocol": true` from the `validFields` map (~line 28). This ensures `${service.protocol}` is rejected as a template variable.

- [ ] **Step 3: Update the error message listing valid fields**

In `config.go` (~line 62), the error message lists valid template fields: `"valid: port, hostname, url, protocol, env_var"`. Remove `protocol` from this list so it reads `"valid: port, hostname, url, env_var"`.

- [ ] **Step 4: Update validation in `validate()`**

Remove the check (~lines 326-328) that requires `protocol: http` or `protocol: https` when hostname is set. The hostname field alone is now sufficient. Keep the hostname format validation and project name check.

- [ ] **Step 5: Update copyService()**

In the `copyService` helper (~line 265), remove `Protocol: rs.Protocol`.

- [ ] **Step 6: Update config tests**

In `config_test.go`:
- Remove `TestLoad_WithProtocol` — tests parsing of protocol field
- Remove `TestHostnameRequiresHTTPProtocol` — validates protocol requirement for hostname
- Remove `TestLoad_ComputedProtocolField` — tests `${web.protocol}` as valid template ref
- Remove `TestResolveComputed_ProtocolField` — tests protocol resolution in computed values
- Update `TestLoad_WithHostname` to remove `protocol: http` from its config YAML and remove protocol assertions
- Update `TestResolveComputed_HostnameReference` and any other tests that include `protocol: http` in their config fixtures — remove the protocol line from YAML
- Grep `config_test.go` for any remaining `protocol` or `Protocol` references and remove them

---

### Task 2: Allocation — Remove Protocol from build and template vars

**Files:**
- Modify: `internal/allocation/allocation.go`
- Modify: `internal/allocation/allocation_test.go`

- [ ] **Step 1: Remove Protocols from Allocation result**

In `allocation.go` `Build()` function: remove `Protocols: computeProtocols(cfg)`.

- [ ] **Step 2: Delete computeProtocols() function**

Remove the entire `computeProtocols()` function.

- [ ] **Step 3: Update BuildTemplateVars()**

- Remove the lines that add `vars[name+".protocol"]`
- Where protocol is read from `svc.Protocol` with fallback to `"http"` for URL construction — replace with just `protocol := "http"` (hostname-bearing services are always HTTP)

- [ ] **Step 4: Update allocation tests**

In `allocation_test.go`:
- Remove `Protocol: "http"` from all `config.Service` struct literals
- Remove assertions on `alloc.Protocols`

---

### Task 3: Registry — Remove Protocols map from Allocation

**Files:**
- Modify: `internal/registry/registry.go`
- Modify: `internal/registry/registry_test.go`

- [ ] **Step 1: Remove Protocols from Allocation struct**

Remove `Protocols map[string]string` from the `Allocation` struct.

- [ ] **Step 2: Remove Protocols map initialization in Load()**

Remove the nil-map initialization for Protocols in the Load function.

- [ ] **Step 3: Update registry tests**

Remove `Protocols: map[string]string{...}` from all test fixtures and assertions.

---

### Task 4: URL utilities — Remove protocol parameter, use hostname presence

**Files:**
- Modify: `internal/urlutil/urlutil.go`
- Modify: `internal/urlutil/urlutil_test.go`

- [ ] **Step 1: Simplify EffectiveScheme()**

Change signature from `EffectiveScheme(protocol, hostname string, httpsEnabled bool)` to `EffectiveScheme(hostname string, httpsEnabled bool)`. Logic: if httpsEnabled and hostname ends in `.test`, return `"https"`, otherwise return `"http"`.

```go
func EffectiveScheme(hostname string, httpsEnabled bool) string {
	if httpsEnabled && strings.HasSuffix(hostname, ".test") {
		return "https"
	}
	return "http"
}
```

- [ ] **Step 2: Simplify ServiceURL()**

Change signature from `ServiceURL(protocol, hostname string, port int, httpsEnabled bool)` to `ServiceURL(hostname string, port int, httpsEnabled bool)`.

**Important:** Return `""` when hostname is empty. This preserves the existing guard semantics — callers check `if url != ""` to determine if a service has a displayable URL. Returning `http://localhost:5432` for postgres would leak URLs into dashboard/status output.

```go
func ServiceURL(hostname string, port int, httpsEnabled bool) string {
	if hostname == "" {
		return ""
	}
	if strings.HasSuffix(hostname, ".test") {
		return fmt.Sprintf("%s://%s", EffectiveScheme(hostname, httpsEnabled), hostname)
	}
	return fmt.Sprintf("http://%s:%d", hostname, port)
}
```

- [ ] **Step 3: Update urlutil tests**

Rewrite tests to remove protocol parameter. Test cases:
- Hostname `.test` with httpsEnabled → `https://foo.test`
- Hostname `.test` without httpsEnabled → `http://foo.test`
- Empty hostname → `""` (no URL)
- Non-`.test` hostname → `http://hostname:{port}`

---

### Task 5: Daemon and dashboard — Remove protocol from routes and API

**Files:**
- Modify: `internal/daemon/routes.go`
- Modify: `internal/daemon/routes_test.go`
- Modify: `internal/daemon/daemon_test.go`
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`
- Modify: `internal/dashboard/health.go` (if it references Protocols)

- [ ] **Step 1: Update buildRoutes() in routes.go**

The loop iterates `alloc.Hostnames` and then checks protocol. After this change, every entry in `alloc.Hostnames` is implicitly HTTP and gets a route. Remove `proto := alloc.Protocols[svcName]` and the `if proto == "http" || proto == "https"` check.

- [ ] **Step 2: Update routes_test.go**

- Remove `Protocols: map[string]string{...}` from all allocation fixtures
- Remove or replace `TestBuildRoutesSkipsNonHTTPProtocols` — the equivalent behavior is now that services without hostnames don't get routes, which is implicit in the `alloc.Hostnames` iteration. This test can be removed if it's redundant with other hostname tests.

- [ ] **Step 3: Update daemon_test.go**

Remove `Protocols: map[string]string{...}` from all allocation fixtures in this file (multiple occurrences).

- [ ] **Step 4: Update dashboard handler.go**

- Remove `Protocol string` from `ServiceJSON` struct
- Remove `protocol := alloc.Protocols[name]` and `sj.Protocol = protocol`
- Update `urlutil.ServiceURL()` calls to new signature: `urlutil.ServiceURL(hostname, port, httpsEnabled)`

- [ ] **Step 5: Update handler_test.go**

Remove `Protocols: map[string]string{...}` from all allocation fixtures. Remove assertions on `.Protocol` in JSON responses.

---

### Task 6: Commands — Update open, share, qr, render, status, init

**Files:**
- Modify: `cmd/open.go`
- Modify: `cmd/share.go`
- Modify: `cmd/qr.go`
- Modify: `cmd/render.go`
- Modify: `cmd/status.go`
- Modify: `cmd/init.go`
- Modify: `cmd/cmd_test.go`

- [ ] **Step 1: Update cmd/render.go**

- Remove `Protocol string` from `svcJSON` struct
- In `buildServiceMap()`: remove `Protocol: svc.Protocol`, update `urlutil.ServiceURL()` call to new signature — pass hostname instead of protocol
- In `serviceURLSuffix()`: update `urlutil.ServiceURL()` call

- [ ] **Step 2: Update cmd/open.go**

Replace all protocol checks with hostname checks. **Important:** The `open` command should only open services that have a hostname. When iterating all services, skip those without a hostname (don't construct `http://localhost:{port}` URLs for infrastructure services).

- Where it checks `svc.Protocol == ""` → check hostname absence (from `alloc.Hostnames`)
- Update `urlutil.EffectiveScheme()` and `urlutil.ServiceURL()` calls to new signatures
- Change error messages from "Add 'protocol: http'" to "Add 'hostname' to services in outport.yml"
- In `openService()` for single-service mode: check hostname not protocol, update error messages
- Remove the `else` branch that constructs URLs for non-hostname services — just skip them

- [ ] **Step 3: Update cmd/share.go**

In `resolveShareServices()`:
- Change `svc.Protocol != "http" && svc.Protocol != "https"` to `svc.Hostname == ""`
- Change `svc.Protocol == "http" || svc.Protocol == "https"` to `svc.Hostname != ""`
- Update all error messages to reference hostname instead of protocol

- [ ] **Step 4: Update cmd/qr.go**

- Replace protocol checks with hostname checks
- Update error messages
- Rename `resolveHTTPServices()` to `resolveWebServices()` since it now checks hostname, not protocol

- [ ] **Step 5: Update cmd/status.go**

- Remove `s.Protocol = svc.Protocol`
- Update `urlutil.ServiceURL()` call to new signature, using hostname from alloc

- [ ] **Step 6: Update cmd/init.go**

Remove `protocol: http` from the init template. The template should show `hostname:` without protocol:

```
services:
  web:
    env_var: PORT
    hostname: %s
```

And the commented multi-service example:
```
#  frontend:
#    env_var: FRONTEND_PORT
#    hostname: app.%s
```

- [ ] **Step 7: Update cmd/cmd_test.go**

This is the largest change. For every reference to protocol:
- Remove `Protocol: "http"` from all `config.Service{}` struct literals (multiple in `TestBuildTemplateVars*` tests)
- Remove `protocol: http` from all YAML config strings (e.g., `testConfigWithHTTP`)
- Update error message assertions ("Add 'protocol: http'" → "Add 'hostname'")
- Remove or rewrite tests that specifically test protocol behavior — convert "service without protocol" tests to "service without hostname" tests
- Remove assertions on `vars["web.protocol"]` or `alloc.Protocols`

Key locations:
- `testConfigWithHTTP` constant
- `TestBuildTemplateVarsNewFields` — remove `Protocol: "http"` and `vars["web.protocol"]` / `vars["db.protocol"]` assertions
- `TestBuildTemplateVarsHTTPS` — remove `Protocol: "http"`
- `TestBuildTemplateVars_TunnelOverrides` — remove `Protocol: "http"`
- `TestBuildTemplateVars_NilTunnelURLs` — remove `Protocol: "http"`
- `TestPrintShareJSON_IncludesComputedValues` — remove `Protocol: "http"`
- `TestShare_ServiceWithoutProtocol` → rename to `TestShare_ServiceWithoutHostname`, update YAML and expected error
- `TestOpen_NoProtocol` → rename to `TestOpen_NoHostname`, update YAML and expected error

- [ ] **Step 8: Run all tests**

Run: `just test`
Expected: All tests pass

- [ ] **Step 9: Run lint**

Run: `just lint`
Expected: 0 issues

---

### Task 7: Documentation — Remove all protocol references

**Files:**
- Modify: `docs/reference/configuration.md`
- Modify: `docs/reference/commands.md`
- Modify: `docs/guide/getting-started.md`
- Modify: `docs/guide/examples.md`
- Modify: `docs/guide/tips.md`
- Modify: `docs/guide/sharing.md`
- Modify: `docs/guide/vscode.md`
- Modify: `docs/guide/work-with-ai.md`
- Modify: `docs/.vitepress/theme/HomeLayout.vue`
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `skills/outport/SKILL.md`

- [ ] **Step 1: Update docs/reference/configuration.md**

- Remove the entire `#### protocol` section
- Update the `#### hostname` section to remove "Only valid for services with `protocol: http` or `protocol: https`" — hostname is now a standalone field
- Remove `protocol: http` from all YAML examples
- Remove `${service.protocol}` from the template syntax table
- Update the full example at the top to remove protocol lines

- [ ] **Step 2: Update docs/reference/commands.md**

- Remove protocol references in the `open` and `share` command descriptions

- [ ] **Step 3: Update docs/guide/getting-started.md**

- Remove `protocol: http` from the example `outport.yml` config
- Verify the `.env` output example still makes sense

- [ ] **Step 4: Update docs/guide/examples.md**

- Remove `protocol: http` from all example YAML configs
- Remove any explanatory text about the protocol field

- [ ] **Step 5: Update docs/guide/tips.md**

- **Remove the entire "protocol: http vs https" section** (lines 21-25) — this section explains when to use `protocol: http` vs `protocol: https`, which is no longer relevant
- Remove any other protocol references

- [ ] **Step 6: Update docs/guide/vscode.md**

- Remove `protocol` from the autocomplete fields list

- [ ] **Step 7: Update docs/guide/work-with-ai.md**

- Remove "protocols" from the list of what the skill covers

- [ ] **Step 8: Update docs/.vitepress/theme/HomeLayout.vue**

- Remove `protocol: http` from the YAML code sample on the homepage

- [ ] **Step 9: Update README.md**

- Remove any protocol references in examples or text

- [ ] **Step 10: Update CLAUDE.md**

- Remove `protocol` from the config description and architecture text

- [ ] **Step 11: Update skills/outport/SKILL.md**

- Remove protocol references from the skill documentation

- [ ] **Step 12: Grep for stragglers**

Run: `grep -ri "protocol" --include="*.md" --include="*.vue" --include="*.ts" docs/ README.md CLAUDE.md skills/`
Expected: No references to the `protocol` config field remain. (References to "HTTP" as a general concept are fine.)

---

### Task 8: Final verification and commit

- [ ] **Step 1: Full test suite**

Run: `just test`
Expected: All tests pass

- [ ] **Step 2: Full lint**

Run: `just lint`
Expected: 0 issues

- [ ] **Step 3: Build**

Run: `just build`
Expected: Clean build

- [ ] **Step 4: Grep for remaining protocol references in Go code**

Run: `grep -rn "\.Protocol\|Protocols\|svc\.Protocol\|alloc\.Protocol" --include="*.go" .`
Expected: Zero matches

- [ ] **Step 5: Docs build**

Run: `cd docs && npm run docs:build`
Expected: Clean build

- [ ] **Step 6: Commit on feature branch and push**

```
git add -A
git commit -m "refactor: remove protocol field — hostname implies web service

The protocol config field was redundant with hostname. If a service
has a hostname, it's HTTP and gets proxied. Simplifies config from
3 fields (env_var + protocol + hostname) to 2 (env_var + hostname).

Removes: protocol from config struct, Protocols map from registry,
\${service.protocol} template variable, all protocol validation."
```

---

### Post-implementation: Clean up Steve's existing outport.yml files

After the refactor is merged, update all `outport.yml` files on Steve's machine to remove `protocol:` lines. This is a separate step — do not include in the refactor commit.

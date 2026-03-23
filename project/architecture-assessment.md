# Architecture Assessment — Outport
Generated: 2026-03-23 | Framework: Go (Cobra CLI)

## Summary

Outport's architecture is fundamentally sound. The `internal/` package structure is clean — each package has a single clear responsibility, matching `_test.go` files, and well-defined boundaries. The foundational decisions are good: build tags for platform isolation (`darwin.go` / `other.go`), a Provider interface for tunnel backend swappability, and idiomatic Cobra usage following GitHub CLI conventions.

The main structural concern is that the `cmd/` layer has grown too thick. Business logic (allocation construction, hostname computation, template variable building) and shared rendering utilities have accumulated in command files — particularly `up.go` at 534 lines. This violates the stated "skinny controller" philosophy: Cobra commands should parse flags, call domain code, and format output. Instead, `up.go` acts as both a command handler and a shared utility library imported by `ports.go`, `status.go`, and `share.go`.

The fixes are mostly mechanical extractions, not redesigns. The `internal/` layer is in good shape and doesn't need restructuring — the work is about pulling domain logic out of `cmd/` and into the right `internal/` packages, and cleaning up inconsistencies in file writes, JSON output, and registry access patterns.

## Findings by Lens

### Structural Organization
**Health: Good**

Clean two-layer layout: `cmd/` for CLI commands, `internal/` for domain packages. Every internal package has a single clear responsibility and a matching `_test.go` file. Platform-specific code is correctly isolated via build tags. No findings.

### Design Patterns
**Health: Good**

Patterns are applied where they earn their weight: Provider interface (tunnel) enables backend swapping, AllocProvider interface decouples the dashboard from the daemon's RouteTable, CertStore wraps a locked cache behind GetCertificate, RouteTable is an Observer subject. The FlagError + ErrSilent sentinel pattern mirrors the GitHub CLI idiom. No cargo-cult abstraction. No findings.

### Composition & Inheritance
**Health: Good**

Go's composition model is used correctly throughout. No inheritance hierarchies. Behavior is composed via interfaces and callbacks. The HealthChecker is wired to the Handler via a callback rather than embedding, keeping both independently testable. No findings.

### Coupling & Cohesion
**Health: Needs Attention**

#### Finding: Domain logic (allocation building, hostname computation) lives in cmd/
- **Severity:** Medium
- **Location:** `cmd/up.go:185-302`
- **What:** `buildAllocation`, `computeHostnames`, `computeProtocols`, `computeEnvVars`, and `buildTemplateVars` are pure domain logic called from 3 commands (up, rename, promote). They transform config + instance + ports into allocations and template maps with no knowledge of CLI flags or output.
- **Why it matters:** Violates the skinny-controller philosophy. New commands that need allocations must import from `up.go`. These functions aren't independently testable outside the `cmd` package.
- **Recommendation:** Extract into `internal/allocation/` or add to `internal/registry/` (since they produce `registry.Allocation` structs). Makes them independently testable and reusable from any command.

#### Finding: `cmd/up.go` has dual responsibility — allocation orchestration + output formatting
- **Severity:** Medium
- **Location:** `cmd/up.go` (534 lines)
- **What:** `up.go` is simultaneously the busiest command and a shared utility library. `printFlatServices`, `printServiceLine`, `printHeader`, `sortedMapKeys`, JSON types (`svcJSON`, `upJSON`, `computedJSON`), and display helpers are called from `ports.go`, `status.go`, and `share.go`.
- **Why it matters:** Debugging output for `outport ports` means reading `up.go`. New contributors copy from the wrong file. The file has two reasons to change: command logic and shared rendering.
- **Recommendation:** Extract shared rendering into `cmd/render.go`. Leave `up.go` as a thin command handler.

#### Finding: Hostname uniqueness check bypasses registry API
- **Severity:** Medium
- **Location:** `cmd/up.go:104-116`
- **What:** Manually iterates `reg.Projects` and reconstructs registry keys by hand (`cfg.Name + "/" + ctx.Instance`) instead of using `registry.Key()`. This is the only place that reconstructs a registry key manually.
- **Why it matters:** If registry storage structure changes, this code silently breaks. The domain question "is this hostname taken?" belongs in the registry package, not command code.
- **Recommendation:** Add `Registry.FindHostname(hostname, excludeKey string) (string, bool)` and call it from `up.go`.

#### Finding: `gc.go` and `status.go` directly mutate/iterate `reg.Projects`
- **Severity:** Low
- **Location:** `cmd/gc.go:28-32`, `cmd/status.go:86-175`
- **What:** `gc.go` uses `delete(reg.Projects, key)` instead of `reg.Remove()`. `status.go` iterates the map directly for both zero-check and display.
- **Why it matters:** Registry.Projects is exported for JSON marshaling but is being treated as a mutable public API. If the registry adds change-tracking or caching, direct mutations bypass it.
- **Recommendation:** Add `Registry.RemoveStale(predicate func(projectDir string) bool) []string` and `Registry.All() map[string]Allocation` snapshot accessor.

#### Finding: `mergeEnvFiles` defined in `rename.go` but shared across cmd package
- **Severity:** Low
- **Location:** `cmd/rename.go:97`, called from `cmd/envfiles.go:97`
- **What:** Core env-writing function lives in `rename.go` despite being called by `writeEnvFiles` in `envfiles.go`, which is itself used by up, down, promote, rename, and share.
- **Why it matters:** When looking for the env writing pipeline, `envfiles.go` is the obvious place — but the core implementation is one file away.
- **Recommendation:** Move `mergeEnvFiles` from `cmd/rename.go` to `cmd/envfiles.go`.

#### Finding: `resolveComputedFromAlloc` called identically from 3 commands with same 6 args
- **Severity:** Low
- **Location:** `cmd/ports.go:68,92`, `cmd/status.go:146,204`, `cmd/up.go:150`
- **What:** The 6-argument pattern appears verbatim in 4+ call sites. Callers always unpack the same fields from `Allocation` plus `httpsEnabled`, with `tunnelURLs` always `nil` except in share.
- **Why it matters:** When `Allocation` gains a new field relevant to template rendering, every call site must be updated manually.
- **Recommendation:** Add a convenience wrapper that accepts `Allocation` + `httpsEnabled` and delegates, reducing 6-arg calls to 3.

### Framework Alignment
**Health: Good**

Cobra is used idiomatically: `RunE` for error-returning handlers, `PersistentPreRun` for cross-cutting hooks, `GroupID` for command grouping, custom `FlagErrorFunc` for usage display. The `ExactArgs`/`NoArgs`/`MaximumArgs` wrappers are clean extensions. `go:embed` for dashboard static files is correct. No findings.

### API & Interface Design
**Health: Needs Attention**

#### Finding: `writeEnvFiles` has 11 positional parameters
- **Severity:** Medium
- **Location:** `cmd/envfiles.go:85-107`
- **What:** Called from 6 sites. Multiple `bool` and `map[string]string` params of the same type can be silently swapped. Every call passes `nil` for `tunnelURLs` except the share command.
- **Why it matters:** Adding any new parameter requires touching all 6 call sites. Parameter reordering bugs between same-typed args are hard to catch at compile time.
- **Recommendation:** Introduce `EnvWriteOptions` struct for the auth/IO group (`AutoApprove`, `ApprovedPaths`, `TunnelURLs`, `Stdin`, `Stderr`). Reduces positional args to 6 with named fields for the ambiguous ones.

#### Finding: `AllocProvider.AllPorts()` snapshot can diverge from `Allocations()`
- **Severity:** Low
- **Location:** `internal/dashboard/handler.go:18-21`, `internal/daemon/routes.go:56-61`
- **What:** `portIndex` rebuild and health checker port list are updated by separate code paths with no shared lock. Under a registry update race, health changes for new ports get silently dropped.
- **Why it matters:** Timing-dependent silent failure in health status reporting. Low risk at 3-second poll intervals but grows if frequency increases.
- **Recommendation:** Collapse into a single `Snapshot()` method or rebuild `portIndex` atomically before notifying the health checker.

#### Finding: `RouteTable.Update` is dead in production
- **Severity:** Low
- **Location:** `internal/daemon/routes.go:34-41`
- **What:** Only called from tests. Production always uses `UpdateWithAllocations`. Exported method gives false impression it's a valid production entry point.
- **Why it matters:** Future contributors may use `Update` in production, silently leaving allocations and ports stale.
- **Recommendation:** Make unexported (`update`) or delete. Tests can use `UpdateWithAllocations(routes, nil)`.

#### Finding: `printServiceLine` boolean flag selects two different layouts
- **Severity:** Low
- **Location:** `cmd/up.go:487-533`
- **What:** `showEnvVar bool` parameter completely changes rendered output. Two branches share only port/URL rendering. Exactly 2 call sites, each hardcoding a constant bool.
- **Why it matters:** Boolean parameters that select between fundamentally different modes are hidden control flow.
- **Recommendation:** Split into `printServiceLineWithEnvVar` and `printServiceLineCompact`. Extract shared URL/hostname suffix logic into a small helper.

### Duplication & Reuse
**Health: Needs Attention**

#### Finding: Dotenv writes are not atomic (registry and tunnel writes are)
- **Severity:** Medium
- **Location:** `internal/dotenv/dotenv.go:134-143`
- **What:** `dotenv.writeLines` calls `os.WriteFile` directly. `registry.Save()` and `tunnel.WriteState()` both use temp-file-then-rename for atomicity. A crash mid-write on a `.env` file leaves it truncated, including user content above the managed block.
- **Why it matters:** `.env` files are the primary output of `outport up`. Inconsistent safety guarantees across the three write paths.
- **Recommendation:** Write to `.tmp` sibling, then `os.Rename`. 3-line change matching existing pattern in `registry.Save` and `tunnel.WriteState`.

#### Finding: `sortedMapKeys` duplicated across cmd and internal packages
- **Severity:** Medium
- **Location:** `cmd/up.go:304-311`, `internal/doctor/project.go:75-79`, `internal/dashboard/handler.go:303-307`
- **What:** Three separate implementations of "get sorted keys from a map."
- **Why it matters:** Unnecessary duplication of a trivial operation that Go 1.23 stdlib handles.
- **Recommendation:** Delete all three. Replace with `slices.Sorted(maps.Keys(m))` (Go 1.23+). Zero new shared code needed.

#### Finding: `writeJSON` exists in `cmdutil.go` but 5 commands inline the pattern
- **Severity:** Low
- **Location:** `cmd/up.go`, `cmd/ports.go`, `cmd/status.go`, `cmd/system_start.go`, `cmd/system.go`
- **What:** `cmdutil.go` exports `writeJSON(cmd, v)` but 5+ commands inline the identical `json.MarshalIndent` + `fmt.Fprintln` sequence.
- **Why it matters:** Inconsistency in a pattern specifically extracted for reuse. Future changes to JSON output behavior require touching 6+ sites.
- **Recommendation:** Replace inlined blocks with calls to `writeJSON`. For `printSystemStatusJSON` (which takes `io.Writer`), add a `writeJSONTo(w, v)` variant.

#### Finding: Port collection from `map[string]int` into `[]int` duplicated
- **Severity:** Low
- **Location:** `cmd/ports.go:102-106`, `cmd/status.go:94-100`
- **What:** Both collect `[]int` from port maps for `portcheck.CheckAll`. Same idiom, slightly different contexts.
- **Recommendation:** Use `maps.Values` (Go 1.23+) or move `checkPorts` to `cmd/cmdutil.go`.

## Prioritized Recommendations

### Quick wins (hours each, low risk)
1. **Make dotenv writes atomic** — Add temp+rename to `dotenv.writeLines`. 3-line change matching existing patterns in registry and tunnel. Prevents `.env` corruption on crash.
2. **Replace `sortedMapKeys` with stdlib** — Delete all 3 implementations, use `slices.Sorted(maps.Keys(m))`. Zero new code.
3. **Consolidate `writeJSON` usage** — Replace 5 inlined `json.MarshalIndent` blocks with calls to the existing `writeJSON` helper. Add `writeJSONTo(w, v)` variant for `io.Writer` callers.
4. **Move `mergeEnvFiles` to `envfiles.go`** — One function move for better cohesion.
5. **Make `RouteTable.Update` unexported** — Tiny API cleanup, update 9 test call sites.
6. **Use `maps.Values` for port collection** — Replace hand-rolled loops with stdlib.

### Medium efforts (a day or two each)
1. **Extract shared rendering into `cmd/render.go`** — Move `printFlatServices`, `printServiceLine`, `printHeader`, `truncate`, `uniformValue`, `buildServiceMap`, `buildComputedMap`, JSON types, and `sortedMapKeys` out of `up.go`. Cuts `up.go` roughly in half.
2. **Introduce `EnvWriteOptions` struct** — Tame the 11-parameter `writeEnvFiles`. Group auth/IO params into a named struct. Update 6 call sites.
3. **Add `Registry.FindHostname()` method** — Encapsulate hostname uniqueness check behind the registry API. Remove direct `reg.Projects` iteration from `up.go`.
4. **Add `Registry.RemoveStale()` and `Registry.All()`** — Stop `gc.go` and `status.go` from directly mutating/iterating the Projects map.

### Strategic restructuring (a week, phased)
1. **Extract domain logic from `cmd/` into `internal/allocation/`** — Move `buildAllocation`, `computeHostnames`, `computeProtocols`, `computeEnvVars`, `buildTemplateVars`, and `resolveComputedFromAlloc` into a new `internal/allocation` package (or distribute across existing `internal/` packages). This is the architectural centerpiece: it establishes the skinny-controller boundary so that command files only parse flags, call domain code, and format output. Should be done after the rendering extraction (medium effort #1) to avoid merge conflicts.

## Action Plan

*Empty until implementation phases are planned. Run `/architect` and request planning to populate.*

# Network Interface Setting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global `[network] interface` setting so users configure their LAN interface once instead of passing `--interface` on every command.

**Architecture:** New `NetworkSettings` struct in the settings package, threaded through the daemon to the dashboard handler, and used as fallback in `cmd/qr.go` when the `--interface` flag is empty. The `lanip` error message is updated to mention the config file.

**Tech Stack:** Go, go-ini/ini

**Spec:** `docs/superpowers/specs/2026-03-26-network-interface-setting-design.md`

---

### Task 1: Add NetworkSettings to the settings package

**Files:**
- Modify: `internal/settings/settings.go:28-35` (Settings struct)
- Modify: `internal/settings/settings.go:66-84` (Defaults func)
- Modify: `internal/settings/settings.go:118-161` (LoadFrom func)
- Modify: `internal/settings/settings.go:182-202` (DefaultConfigContent func)
- Test: `internal/settings/settings_test.go`

- [ ] **Step 1: Write the failing test for loading interface from config**

Add to `internal/settings/settings_test.go`:

```go
func TestLoadNetworkInterface(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(`
[network]
interface = en0
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Network.Interface != "en0" {
		t.Errorf("interface = %q, want %q", s.Network.Interface, "en0")
	}
}

func TestDefaultNetworkInterfaceIsEmpty(t *testing.T) {
	s := Defaults()
	if s.Network.Interface != "" {
		t.Errorf("default interface = %q, want empty", s.Network.Interface)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/settings/ -run "TestLoadNetworkInterface|TestDefaultNetworkInterface" -v`
Expected: FAIL — `s.Network` doesn't exist yet.

- [ ] **Step 3: Implement NetworkSettings**

In `internal/settings/settings.go`:

1. Add the struct after `DNSSettings` (after line 64):

```go
// NetworkSettings controls how Outport detects the machine's LAN IP address,
// used for QR codes and the dashboard's LAN URL display.
type NetworkSettings struct {
	// Interface is the network interface name (e.g., "en0", "eth0", "wlan0")
	// used for LAN IP detection. When empty, Outport auto-detects by scanning
	// common interface names. Set via the "interface" key in the [network] section.
	// Default: empty (auto-detect).
	Interface string
}
```

2. Add `Network NetworkSettings` to the `Settings` struct (after `Tunnels TunnelSettings`, line 34):

```go
// Network contains settings for LAN IP detection.
Network NetworkSettings
```

3. No change to `Defaults()` — the zero value for `Interface` is `""` which means auto-detect, which is correct.

4. Add parsing in `LoadFrom`, after the tunnels section (after line 155):

```go
network := cfg.Section("network")
if key, err := network.GetKey("interface"); err == nil {
	s.Network.Interface = key.String()
}
```

5. Update `DefaultConfigContent()` — append before the closing backtick (include a leading blank line for visual consistency with the spacing between existing sections):

```
[network]
# Network interface for LAN IP detection (e.g., en0, eth0, wlan0).
# Used by QR codes and the dashboard to show your LAN address.
# When unset, Outport auto-detects by scanning common interface names.
# interface = en0
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/settings/ -v`
Expected: ALL PASS (including new tests and existing `TestDefaultConfigContentRoundTrips`).

- [ ] **Step 5: Commit**

```bash
git add internal/settings/settings.go internal/settings/settings_test.go
git commit -m "feat: add [network] interface setting to global config"
```

---

### Task 2: Thread interface setting through daemon to dashboard

**Files:**
- Modify: `internal/daemon/daemon.go:32-78` (DaemonConfig struct)
- Modify: `internal/daemon/daemon.go:99-151` (New func)
- Modify: `internal/dashboard/handler.go:132-143` (Handler struct)
- Modify: `internal/dashboard/handler.go:155-182` (NewHandler func)
- Modify: `internal/dashboard/handler.go:305-322` (refreshCaches func)
- Modify: `cmd/daemon.go:42-48` (DaemonConfig construction)

- [ ] **Step 1: Add networkInterface field to dashboard Handler**

In `internal/dashboard/handler.go`:

1. Add field to `Handler` struct (after `cachedLANIP`, line 141):

```go
networkInterface string                         // configured LAN interface override
```

2. Add `networkInterface string` parameter to `NewHandler` (line 155):

```go
func NewHandler(provider AllocProvider, httpsEnabled bool, version string, healthInterval time.Duration, networkInterface string) *Handler {
```

3. Store it in the handler (line 162, after `version: version,`):

```go
networkInterface: networkInterface,
```

4. Use it in `refreshCaches` (line 308, replace `lanip.Detect("")`):

```go
if ip, err := lanip.Detect(h.networkInterface); err == nil {
```

- [ ] **Step 2: Add NetworkInterface to DaemonConfig**

In `internal/daemon/daemon.go`, add field to `DaemonConfig` (after `HealthInterval`, line 77):

```go
// NetworkInterface is the network interface name for LAN IP detection
// (e.g., "en0"). When empty, the dashboard auto-detects the LAN interface.
// Configurable via the global settings file.
NetworkInterface string
```

- [ ] **Step 3: Pass it through in daemon.New**

In `internal/daemon/daemon.go`, update the `NewHandler` call (line 110):

```go
dashHandler := dashboard.NewHandler(dashProvider, httpsEnabled, cfg.Version, healthInterval, cfg.NetworkInterface)
```

- [ ] **Step 4: Wire it in cmd/daemon.go**

In `cmd/daemon.go`, add to the `DaemonConfig` construction (after `HealthInterval`, line 47):

```go
NetworkInterface: s.Network.Interface,
```

- [ ] **Step 5: Build to verify it compiles**

Run: `go build ./...`
Expected: Success, no errors.

- [ ] **Step 6: Verify the dashboard passes the interface through**

The key behavioral change is that `refreshCaches` now calls `lanip.Detect(h.networkInterface)` instead of `lanip.Detect("")`. Since `refreshCaches` is called from `NewHandler`, `OnRegistryUpdate`, and `onHealthChange`, all three paths are covered by storing the value on the struct. Verify with a quick grep:

Run: `grep -n 'lanip.Detect' internal/dashboard/handler.go`
Expected: Shows `lanip.Detect(h.networkInterface)` — confirms the hardcoded `""` is gone.

- [ ] **Step 7: Commit**

```bash
git add internal/dashboard/handler.go internal/daemon/daemon.go cmd/daemon.go
git commit -m "feat: thread network interface setting from config to dashboard"
```

---

### Task 3: Use global setting as fallback in outport qr

**Files:**
- Modify: `cmd/qr.go:38-76` (runQR func)

- [ ] **Step 1: Load settings and use as fallback**

In `cmd/qr.go`:

1. Add import for the settings package:

```go
"github.com/steveclarke/outport/internal/settings"
```

2. In `runQR`, after `loadProjectContext()` (after line 42), load settings and resolve the interface:

```go
iface := qrInterfaceFlag
if iface == "" {
	s, err := settings.Load()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}
	iface = s.Network.Interface
}
```

3. In `printLANQR`, change the signature to accept the interface string:

```go
func printLANQR(cmd *cobra.Command, services map[string]int, networkInterface string) error {
	ip, err := lanip.Detect(networkInterface)
```

4. Update the call site in `runQR` (line 75):

```go
return printLANQR(cmd, services, iface)
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./...`
Expected: Success, no errors.

- [ ] **Step 3: Verify the priority chain logic**

The priority chain (flag > setting > auto-detect) is implemented in `runQR`. Verify the logic is correct by reading the code:

1. When `qrInterfaceFlag` is set (e.g., `--interface en1`), it is used directly — `settings.Load()` is never called.
2. When `qrInterfaceFlag` is empty, settings are loaded and `s.Network.Interface` is used.
3. When both are empty, `lanip.Detect("")` auto-detects — this is the existing behavior.

Run: `grep -A10 'iface := qrInterfaceFlag' cmd/qr.go`
Expected: Shows the fallback logic matching the three cases above.

- [ ] **Step 4: Commit**

```bash
git add cmd/qr.go
git commit -m "feat: outport qr falls back to global network interface setting"
```

---

### Task 4: Update lanip error message

**Files:**
- Modify: `internal/lanip/lanip.go:76`
- Test: `internal/lanip/lanip_test.go`

- [ ] **Step 1: Update the error message**

In `internal/lanip/lanip.go`, line 76, change:

```go
return nil, fmt.Errorf("no suitable LAN interface found. Use --interface to specify one")
```

to:

```go
return nil, fmt.Errorf("no suitable LAN interface found. Set [network] interface in ~/.config/outport/config, or use --interface")
```

- [ ] **Step 2: Run existing lanip tests**

Run: `go test ./internal/lanip/ -v`
Expected: ALL PASS (no test checks the exact error string).

- [ ] **Step 3: Commit**

```bash
git add internal/lanip/lanip.go
git commit -m "fix: lanip error message mentions global config as alternative to --interface"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `docs/reference/commands.md` (qr command section)
- Modify: `docs/guide/configuration.md` (if it exists, add network section)

- [ ] **Step 1: Check what docs need updating**

Look at `docs/reference/commands.md` QR section and any configuration guide.

- [ ] **Step 2: Update commands.md QR section**

Add a note about the global setting to the `--interface` flag description — something like:
"Overrides the `[network] interface` global setting for this invocation."

- [ ] **Step 3: Update configuration docs**

If there's a configuration reference page, add the `[network]` section. If not, the `DefaultConfigContent()` is self-documenting.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs: document network interface global setting"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `just test`
Expected: ALL PASS.

- [ ] **Step 2: Run linter**

Run: `just lint`
Expected: No errors.

- [ ] **Step 3: Manual smoke test**

Run: `just build && dist/outport qr` (without --interface) to verify it still works with auto-detect.

- [ ] **Step 4: Verify DefaultConfigContent round-trips**

The existing `TestDefaultConfigContentRoundTrips` test covers this — confirm it passed in step 1.

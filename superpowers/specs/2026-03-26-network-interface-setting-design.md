# Network Interface Global Setting

**Date:** 2026-03-26
**Status:** Approved

## Problem

Outport auto-detects the LAN IP by scanning network interfaces (en0, en1, eth0, wlan0). On machines with multiple interfaces — Ethernet, Wi-Fi, VLANs, VPN adapters — auto-detection often picks the wrong one. Today the only workaround is `outport qr --interface en0`, which must be typed every time. The dashboard has no override at all.

## Solution

Add a `[network]` section to the global settings file (`~/.config/outport/config`) with an `interface` key. When set, all LAN IP detection uses the specified interface instead of auto-detecting.

### Priority chain

1. `--interface` CLI flag (highest, per-invocation override)
2. Global config `[network] interface` (set once, applies everywhere)
3. Auto-detect (existing behavior, unchanged)

A future project-level slot (`outport.yml` / `outport.local.yml`) will insert between the flag and the global config when that work lands. No project-level changes are in scope here — the `outport.local.yml` merging work will handle that.

### Config file addition

```ini
[network]
# Network interface for LAN IP detection (e.g., en0, eth0, wlan0).
# Used by QR codes and the dashboard to show your LAN address.
# When unset, Outport auto-detects by scanning common interface names.
# interface = en0
```

### Consumers

Two places call `lanip.Detect()` today:

1. **`cmd/qr.go`** — Currently uses `--interface` flag, falls back to auto-detect. After: flag > global setting > auto-detect.
2. **`internal/dashboard/handler.go`** (`refreshCaches`) — Currently always auto-detects with no override. After: global setting > auto-detect. The daemon already loads settings at startup and passes values as parameters, so this follows the existing pattern.

## Changes

### `internal/settings/settings.go`
- Add `NetworkSettings` struct with `Interface string` field
- Add `Network NetworkSettings` to `Settings` struct
- Parse `[network]` section in `LoadFrom`
- No validation needed — `lanip.Detect()` already returns a clear error for invalid interface names
- Update `DefaultConfigContent()` with the commented-out `[network]` section

### `cmd/qr.go`
- Load global settings
- When `--interface` flag is empty, fall back to `settings.Network.Interface`
- Pass the resolved interface to `lanip.Detect()` (no change to Detect's API)

### `internal/dashboard/handler.go`
- Add an `interface` string parameter to `NewHandler`, store on `Handler` struct
- Use it in `refreshCaches` when calling `lanip.Detect()` — this covers all call sites (startup, registry updates, and health ticks)

### `internal/daemon/daemon.go`
- Add `NetworkInterface string` field to `DaemonConfig`
- Pass it through to `dashboard.NewHandler`

### `cmd/daemon.go`
- Populate `DaemonConfig.NetworkInterface` from `settings.Network.Interface`

### `cmd/setup.go`
- No direct changes — `DefaultConfigContent()` update covers it

### `internal/lanip/lanip.go`
- Update the "no suitable LAN interface found" error message to mention the global config as an alternative to `--interface`

### Tests
- Settings parsing: verify `[network] interface` is loaded, empty string default (Go zero value, intentional)
- Flag-vs-setting priority in `cmd/qr.go`: flag wins over setting, setting wins over empty
- Dashboard handler: verify the interface parameter is passed through to `lanip.Detect()`

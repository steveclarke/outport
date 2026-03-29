//go:build !linux

package doctor

// DNSChainWarnings returns nil on non-Linux platforms.
// macOS uses /etc/resolver/ per-domain files which don't have
// the systemd-resolved dependency chain.
func DNSChainWarnings() []Result { return nil }

// linuxDNSChecks returns no checks on non-Linux platforms.
func linuxDNSChecks() []Check { return nil }

//go:build !linux

package doctor

// linuxBrowserTrustChecks returns no checks on non-Linux platforms.
// macOS handles browser trust through the system Keychain.
func linuxBrowserTrustChecks() []Check { return nil }

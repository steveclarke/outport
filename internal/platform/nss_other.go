//go:build !linux

package platform

// TrustBrowserCAs is a no-op on non-Linux platforms.
// macOS handles browser trust through the system Keychain.
func TrustBrowserCAs(_ string) []string { return nil }

// UntrustBrowserCAs is a no-op on non-Linux platforms.
func UntrustBrowserCAs() {}

// HasCertutil returns false on non-Linux platforms.
func HasCertutil() bool { return false }

// CertutilInstallHint returns an empty string on non-Linux platforms.
func CertutilInstallHint() string { return "" }

// IsNSSTrusted returns false on non-Linux platforms.
func IsNSSTrusted(_ string) bool { return false }

// FindNSSDatabases returns nil on non-Linux platforms.
func FindNSSDatabases() []NSSDatabase { return nil }

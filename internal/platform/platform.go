package platform

// IsSetup checks if the DNS/proxy infrastructure is configured.
// Returns true if both the resolver file and LaunchAgent plist exist.
func IsSetup() bool {
	return isResolverInstalled() && isPlistInstalled()
}

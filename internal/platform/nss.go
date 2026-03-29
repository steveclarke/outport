package platform

// NSSDatabase represents a discovered NSS certificate database used by
// browsers like Chrome and Firefox on Linux.
type NSSDatabase struct {
	Path        string
	Description string
}

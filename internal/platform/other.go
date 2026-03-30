//go:build !darwin && !linux

package platform

import "fmt"

var errUnsupported = fmt.Errorf("outport system start is only supported on macOS")

const (
	ResolverPath    = "/etc/resolver/test"
	ResolverContent = "nameserver 127.0.0.1\nport 15353\n"
)

func isResolverInstalled() bool { return false }
func isPlistInstalled() bool    { return false }

func PlistPath() string                 { return "" }
func WriteResolverFile() error          { return errUnsupported }
func RemoveResolverFile() error         { return errUnsupported }
func WritePlist(_ string) error  { return errUnsupported }
func RemovePlist() error         { return errUnsupported }
func IsAgentLoaded() bool        { return false }
func LoadAgent() error           { return errUnsupported }
func UnloadAgent() error         { return errUnsupported }
func GeneratePlist(_ string) string { return "" }
func TrustCA(_ string) error            { return errUnsupported }
func UntrustCA(_ string) error          { return errUnsupported }
func IsCATrusted(_ string) bool         { return false }
func EnsurePrivilegedPorts(_ string) error { return errUnsupported }
func ServiceDescription() string            { return "service" }
func ResolverDescription() string           { return "resolver config" }

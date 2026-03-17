//go:build !darwin

package platform

import "fmt"

var errUnsupported = fmt.Errorf("outport setup is only supported on macOS")

func isResolverInstalled() bool { return false }
func isPlistInstalled() bool    { return false }

func WriteResolverFile() error          { return errUnsupported }
func RemoveResolverFile() error         { return errUnsupported }
func WritePlist(_ string) error         { return errUnsupported }
func RemovePlist() error                { return errUnsupported }
func IsAgentLoaded() bool               { return false }
func LoadAgent() error                  { return errUnsupported }
func UnloadAgent() error                { return errUnsupported }
func GeneratePlist(_ string) string     { return "" }
func TrustCA(_ string) error            { return errUnsupported }
func UntrustCA(_ string) error          { return errUnsupported }

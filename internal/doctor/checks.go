package doctor

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
)

// checkFileExists returns Pass if the file at path exists, Fail otherwise.
func checkFileExists(path, name, fix string) *Result {
	if _, err := os.Stat(path); err != nil {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("%s not found", name),
			Fix:     fix,
		}
	}
	return &Result{
		Name:    name,
		Status:  Pass,
		Message: fmt.Sprintf("%s exists", name),
	}
}

// checkResolverContent returns Pass if the file at path exists and has the expected content.
func checkResolverContent(path, expected string) *Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{
			Name:    "Resolver content",
			Status:  Fail,
			Message: "resolver file not found",
			Fix:     "Run: outport system start",
		}
	}
	if string(data) != expected {
		return &Result{
			Name:    "Resolver content",
			Status:  Fail,
			Message: "resolver file has unexpected content",
			Fix:     "Run: outport system start",
		}
	}
	return &Result{
		Name:    "Resolver content",
		Status:  Pass,
		Message: "resolver content is correct",
	}
}

// parsePlistBinaryPath extracts the first string from ProgramArguments in a plist XML.
// Uses token-based XML parsing to preserve positional correspondence between keys and values.
func parsePlistBinaryPath(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "key" {
			continue
		}
		var keyText string
		if err := decoder.DecodeElement(&keyText, &start); err != nil {
			return ""
		}
		if keyText != "ProgramArguments" {
			continue
		}
		// Next start element should be <array>
		for {
			tok, err = decoder.Token()
			if err != nil {
				return ""
			}
			if arr, ok := tok.(xml.StartElement); ok {
				if arr.Name.Local != "array" {
					return ""
				}
				// Read first <string> inside the array
				for {
					tok, err = decoder.Token()
					if err != nil {
						return ""
					}
					if s, ok := tok.(xml.StartElement); ok {
						if s.Name.Local == "string" {
							var val string
							if err := decoder.DecodeElement(&val, &s); err != nil {
								return ""
							}
							return val
						}
					}
				}
			}
		}
	}
}

// checkPlistBinary reads the plist, extracts the binary path from ProgramArguments,
// and verifies the binary exists on disk.
func checkPlistBinary(plistPath string) *Result {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return &Result{
			Name:    "Plist binary",
			Status:  Fail,
			Message: "could not read plist file",
			Fix:     "Run: outport system start",
		}
	}

	binPath := parsePlistBinaryPath(data)
	if binPath == "" {
		return &Result{
			Name:    "Plist binary",
			Status:  Fail,
			Message: "could not parse binary path from plist",
			Fix:     "Run: outport system start",
		}
	}

	if _, err := os.Stat(binPath); err != nil {
		return &Result{
			Name:    "Plist binary",
			Status:  Fail,
			Message: fmt.Sprintf("binary not found: %s", binPath),
			Fix:     "Reinstall outport and run: outport system start",
		}
	}

	return &Result{
		Name:    "Plist binary",
		Status:  Pass,
		Message: fmt.Sprintf("binary exists: %s", binPath),
	}
}

// checkAgentLoaded returns Pass if the LaunchAgent is loaded.
func checkAgentLoaded() *Result {
	if platform.IsAgentLoaded() {
		return &Result{
			Name:    "Agent loaded",
			Status:  Pass,
			Message: "daemon agent is loaded",
		}
	}
	return &Result{
		Name:    "Agent loaded",
		Status:  Fail,
		Message: "daemon agent is not loaded",
		Fix:     "Run: outport system start",
	}
}

// checkPortUp returns Pass if the given port is accepting connections.
func checkPortUp(port int, name, fix string) *Result {
	if portcheck.IsUp(port) {
		return &Result{
			Name:    name,
			Status:  Pass,
			Message: fmt.Sprintf("port %d is up", port),
		}
	}
	return &Result{
		Name:    name,
		Status:  Fail,
		Message: fmt.Sprintf("port %d is not responding", port),
		Fix:     fix,
	}
}

// checkCertExpiry reads a PEM certificate and checks its expiry.
// Returns Warn if expiring within 30 days, Fail if expired or unreadable.
func checkCertExpiry(certPath string) *Result {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return &Result{
			Name:    "CA expiry",
			Status:  Fail,
			Message: "could not read CA certificate",
			Fix:     "Run: outport system start",
		}
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return &Result{
			Name:    "CA expiry",
			Status:  Fail,
			Message: "CA certificate is not valid PEM",
			Fix:     "Run: outport system uninstall && outport system start",
		}
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return &Result{
			Name:    "CA expiry",
			Status:  Fail,
			Message: "could not parse CA certificate",
			Fix:     "Run: outport system uninstall && outport system start",
		}
	}

	now := time.Now()
	if now.After(cert.NotAfter) {
		return &Result{
			Name:    "CA expiry",
			Status:  Fail,
			Message: fmt.Sprintf("CA certificate expired on %s", cert.NotAfter.Format("2006-01-02")),
			Fix:     "Run: outport system uninstall && outport system start",
		}
	}

	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	if daysLeft < 30 {
		return &Result{
			Name:    "CA expiry",
			Status:  Warn,
			Message: fmt.Sprintf("CA certificate expires in %d days", daysLeft),
			Fix:     "Run: outport system uninstall && outport system start",
		}
	}

	return &Result{
		Name:    "CA expiry",
		Status:  Pass,
		Message: fmt.Sprintf("CA certificate valid until %s", cert.NotAfter.Format("2006-01-02")),
	}
}

// checkCATrusted returns Pass if the CA certificate is trusted by the system.
func checkCATrusted(certPath string) *Result {
	if platform.IsCATrusted(certPath) {
		return &Result{
			Name:    "CA trusted",
			Status:  Pass,
			Message: "CA certificate is trusted",
		}
	}
	return &Result{
		Name:    "CA trusted",
		Status:  Fail,
		Message: "CA certificate is not trusted",
		Fix:     "Run: outport system start",
	}
}

// checkRegistryValid checks that the registry file exists and is parseable.
// Missing registry is Warn (normal for fresh installs), corrupt is Fail.
func checkRegistryValid(path string) *Result {
	if _, err := os.Stat(path); err != nil {
		return &Result{
			Name:    "Registry",
			Status:  Warn,
			Message: "registry file not found (normal for fresh installs)",
		}
	}

	if _, err := registry.Load(path); err != nil {
		return &Result{
			Name:    "Registry",
			Status:  Fail,
			Message: fmt.Sprintf("registry is corrupt: %v", err),
			Fix:     "Remove the registry file and re-run outport up in your projects",
		}
	}

	return &Result{
		Name:    "Registry",
		Status:  Pass,
		Message: "registry is valid",
	}
}

// checkCloudflared returns Pass if cloudflared is on the PATH.
func checkCloudflared() *Result {
	path, err := exec.LookPath("cloudflared")
	if err != nil {
		return &Result{
			Name:    "cloudflared",
			Status:  Warn,
			Message: "cloudflared not found (needed for outport share)",
			Fix:     "Install cloudflared: brew install cloudflare/cloudflare/cloudflared",
		}
	}
	return &Result{
		Name:    "cloudflared",
		Status:  Pass,
		Message: fmt.Sprintf("cloudflared found: %s", path),
	}
}

// SystemChecks returns all system-level health checks in order.
func SystemChecks() []Check {
	plistPath := platform.PlistPath()

	caCertPath, _ := certmanager.CACertPath()
	caKeyPath, _ := certmanager.CAKeyPath()
	registryPath, _ := registry.DefaultPath()

	return []Check{
		{
			Name:     "Resolver file",
			Category: "DNS",
			Run: func() *Result {
				return checkFileExists(platform.ResolverPath, "Resolver file", "Run: outport system start")
			},
		},
		{
			Name:     "Resolver content",
			Category: "DNS",
			Run: func() *Result {
				return checkResolverContent(platform.ResolverPath, platform.ResolverContent)
			},
		},
		{
			Name:     "Plist installed",
			Category: "Daemon",
			Run: func() *Result {
				return checkFileExists(plistPath, "Plist installed", "Run: outport system start")
			},
		},
		{
			Name:     "Plist binary",
			Category: "Daemon",
			Run: func() *Result {
				return checkPlistBinary(plistPath)
			},
		},
		{
			Name:     "Agent loaded",
			Category: "Daemon",
			Run: func() *Result {
				return checkAgentLoaded()
			},
		},
		{
			Name:     "HTTP proxy",
			Category: "Daemon",
			Run: func() *Result {
				return checkPortUp(80, "HTTP proxy", "Run: outport system start")
			},
		},
		{
			Name:     "HTTPS proxy",
			Category: "Daemon",
			Run: func() *Result {
				return checkPortUp(443, "HTTPS proxy", "Run: outport system start")
			},
		},
		{
			Name:     "CA cert exists",
			Category: "TLS",
			Run: func() *Result {
				return checkFileExists(caCertPath, "CA cert exists", "Run: outport system start")
			},
		},
		{
			Name:     "CA key exists",
			Category: "TLS",
			Run: func() *Result {
				return checkFileExists(caKeyPath, "CA key exists", "Run: outport system start")
			},
		},
		{
			Name:     "CA expiry",
			Category: "TLS",
			Run: func() *Result {
				return checkCertExpiry(caCertPath)
			},
		},
		{
			Name:     "CA trusted",
			Category: "TLS",
			Run: func() *Result {
				return checkCATrusted(caCertPath)
			},
		},
		{
			Name:     "Registry valid",
			Category: "Registry",
			Run: func() *Result {
				return checkRegistryValid(registryPath)
			},
		},
		{
			Name:     "cloudflared",
			Category: "Tools",
			Run: func() *Result {
				return checkCloudflared()
			},
		},
	}
}

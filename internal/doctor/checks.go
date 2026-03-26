// checks.go contains the individual diagnostic check functions used by the
// "outport doctor" command. Each function tests one specific aspect of the
// system (file existence, port liveness, certificate validity, etc.) and
// returns a Result with a pass/warn/fail status. When a check fails, the
// Result includes a Fix string with the command the user should run.

package doctor

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/platform"
	"github.com/steveclarke/outport/internal/portcheck"
	"github.com/steveclarke/outport/internal/registry"
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

// checkPlistBinary reads the service file, extracts the binary path, and verifies it exists on disk.
// Handles both plist XML (macOS) and systemd unit (Linux) formats.
func checkPlistBinary(plistPath string) *Result {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return &Result{
			Name:    "Service binary",
			Status:  Fail,
			Message: "could not read service file",
			Fix:     "Run: outport system start",
		}
	}

	// On Linux, the service file is a systemd unit (INI format), not a plist (XML).
	if !bytes.Contains(data, []byte("<?xml")) {
		return checkServiceBinary(data)
	}

	binPath := parsePlistBinaryPath(data)
	if binPath == "" {
		return &Result{
			Name:    "Service binary",
			Status:  Fail,
			Message: "could not parse binary path from plist",
			Fix:     "Run: outport system start",
		}
	}

	if _, err := os.Stat(binPath); err != nil {
		return &Result{
			Name:    "Service binary",
			Status:  Fail,
			Message: fmt.Sprintf("daemon plist references missing binary: %s", binPath),
			Fix:     "Run: outport system restart",
		}
	}

	return &Result{
		Name:    "Service binary",
		Status:  Pass,
		Message: fmt.Sprintf("binary exists: %s", binPath),
	}
}

// checkServiceBinary extracts the binary path from a systemd unit's ExecStart line.
func checkServiceBinary(data []byte) *Result {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ExecStart=") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(line, "ExecStart="))
		if len(parts) == 0 {
			break
		}
		binPath := parts[0]
		if _, err := os.Stat(binPath); err != nil {
			return &Result{
				Name:    "Service binary",
				Status:  Fail,
				Message: fmt.Sprintf("service references missing binary: %s", binPath),
				Fix:     "Run: outport system restart",
			}
		}
		return &Result{
			Name:    "Service binary",
			Status:  Pass,
			Message: fmt.Sprintf("binary exists: %s", binPath),
		}
	}
	return &Result{
		Name:    "Service binary",
		Status:  Fail,
		Message: "could not parse binary path from service file",
		Fix:     "Run: outport system start",
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

// SystemChecks returns all system-level health checks in execution order.
// These checks verify the overall Outport infrastructure: DNS resolver
// configuration, daemon process health, TLS certificate state, registry
// integrity, and optional tool availability.
//
// The checks are organized into categories:
//   - DNS: Resolver file exists, has correct content, and resolves *.test domains.
//   - Daemon: LaunchAgent plist exists, references a valid binary, agent is loaded,
//     and HTTP/HTTPS proxy ports (80/443) are listening.
//   - TLS: CA certificate and private key exist, certificate is not expired or
//     expiring soon, and the CA is trusted by the macOS system keychain.
//   - Registry: The registry.json file is parseable (missing is OK for fresh installs).
//   - Tools: Optional dependencies like cloudflared are available.
//
// The returned checks are meant to be passed to a Runner for sequential execution.
func SystemChecks() []Check {
	plistPath := platform.PlistPath()

	caCertPath, _ := certmanager.CACertPath()
	caKeyPath, _ := certmanager.CAKeyPath()
	registryPath, _ := registry.DefaultPath()

	return []Check{
		{
			Name:     "DNS resolver",
			Category: "DNS",
			Run: func() *Result {
				return checkFileExists(platform.ResolverPath, "DNS resolver", "Run: outport system start")
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
			Name:     "DNS resolving *.test",
			Category: "DNS",
			Run: func() *Result {
				return checkDNSResolving("127.0.0.1:15353")
			},
		},
		{
			Name:     platform.ServiceDescription() + " file",
			Category: "Daemon",
			Run: func() *Result {
				return checkFileExists(plistPath, platform.ServiceDescription()+" file", "Run: outport system start")
			},
		},
		{
			Name:     "Service binary",
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
			Name:     "CA certificate",
			Category: "TLS",
			Run: func() *Result {
				return checkFileExists(caCertPath, "CA certificate", "Run: outport system start")
			},
		},
		{
			Name:     "CA private key",
			Category: "TLS",
			Run: func() *Result {
				return checkFileExists(caKeyPath, "CA private key", "Run: outport system start")
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

//go:build linux

package doctor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	resolvConfPath   = "/etc/resolv.conf"
	resolvedStubPath = "/run/systemd/resolve/stub-resolv.conf"
	resolvedConfDir  = "/etc/systemd/resolved.conf.d"
	resolvedConfFile = "/etc/systemd/resolved.conf"
	stubListenerAddr = "127.0.0.53"
	stubListenerDNS  = stubListenerAddr + ":53"
)

// DNSChainWarnings runs the resolv.conf and stub listener checks and returns
// warning messages for any failures. Called by system start to surface DNS
// chain issues at setup time rather than after the user debugs manually.
// Returns nil if both checks pass.
func DNSChainWarnings() []string {
	var warnings []string
	for _, res := range []*Result{
		checkResolvConfRouting(resolvConfPath),
		checkDNSStubListener(stubListenerDNS),
	} {
		if res.Status == Fail {
			msg := res.Message
			if res.Fix != "" {
				msg += "\n      " + res.Fix
			}
			warnings = append(warnings, msg)
		}
	}
	return warnings
}

// linuxDNSChecks returns Linux-specific checks that verify the full DNS
// resolution chain from applications through systemd-resolved to outport.
func linuxDNSChecks() []Check {
	return []Check{
		{
			Name:     "resolv.conf routing",
			Category: "DNS",
			Run:      func() *Result { return checkResolvConfRouting(resolvConfPath) },
		},
		{
			Name:     "DNS stub listener",
			Category: "DNS",
			Run:      func() *Result { return checkDNSStubListener(stubListenerDNS) },
		},
		{
			Name:     "End-to-end DNS",
			Category: "DNS",
			Run:      func() *Result { return checkEndToEndDNS() },
		},
	}
}

// checkResolvConfRouting verifies /etc/resolv.conf routes DNS through the
// systemd-resolved stub listener at 127.0.0.53.
func checkResolvConfRouting(path string) *Result {
	name := "resolv.conf routing"
	fix := "Run: sudo ln -sf " + resolvedStubPath + " /etc/resolv.conf"

	// Check if it's a symlink to the stub
	target, err := os.Readlink(path)
	if err == nil && target == resolvedStubPath {
		return &Result{Name: name, Status: Pass, Message: "resolv.conf routes through systemd-resolved stub"}
	}

	// Not the right symlink — check if 127.0.0.53 is at least a nameserver
	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Name: name, Status: Fail, Message: "could not read /etc/resolv.conf", Fix: fix}
	}

	if containsNameserver(data, stubListenerAddr) {
		return &Result{Name: name, Status: Pass, Message: "resolv.conf contains nameserver " + stubListenerAddr}
	}

	// It's broken — identify who manages it
	manager := identifyResolvConfManager(target, data)
	msg := "resolv.conf does not route through systemd-resolved"
	if manager != "" {
		msg = fmt.Sprintf("resolv.conf is managed by %s, bypassing systemd-resolved", manager)
	}

	return &Result{Name: name, Status: Fail, Message: msg, Fix: fix}
}

// checkDNSStubListener verifies systemd-resolved's stub listener is reachable
// by sending a DNS query to 127.0.0.53:53.
func checkDNSStubListener(addr string) *Result {
	name := "DNS stub listener"

	_, err := dnsProbe(addr, "localhost.")
	if err == nil {
		return &Result{Name: name, Status: Pass, Message: "systemd-resolved stub listener is reachable"}
	}

	// Stub is down — check if DNSStubListener=no is configured
	file := findStubListenerDisabled()
	if file != "" {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: fmt.Sprintf("stub listener disabled (DNSStubListener=no in %s)", filepath.Base(file)),
			Fix:     fmt.Sprintf("Remove %s and run: sudo systemctl restart systemd-resolved", file),
		}
	}

	return &Result{
		Name:    name,
		Status:  Fail,
		Message: "systemd-resolved stub listener (127.0.0.53) is not responding",
		Fix:     "Run: sudo systemctl restart systemd-resolved",
	}
}

// checkEndToEndDNS resolves outport-check.test via the system resolver to
// verify the full chain works end-to-end.
func checkEndToEndDNS() *Result {
	name := "End-to-end DNS"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupHost(ctx, "outport-check.test")
	if err != nil {
		return &Result{
			Name:    name,
			Status:  Fail,
			Message: "could not resolve outport-check.test through system resolver",
			Fix:     "Fix the resolv.conf and stub listener issues above, then run: outport system restart",
		}
	}

	for _, addr := range addrs {
		if addr == "127.0.0.1" {
			return &Result{Name: name, Status: Pass, Message: ".test domains resolve end-to-end"}
		}
	}

	return &Result{
		Name:    name,
		Status:  Fail,
		Message: fmt.Sprintf("outport-check.test resolved to %s (expected 127.0.0.1)", strings.Join(addrs, ", ")),
		Fix:     "Run: outport system restart",
	}
}

// containsNameserver checks if the resolv.conf content has a nameserver line
// matching the given address.
func containsNameserver(data []byte, addr string) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == addr {
				return true
			}
		}
	}
	return false
}

// identifyResolvConfManager identifies what tool is managing resolv.conf from
// the symlink target and file content headers.
func identifyResolvConfManager(symlinkTarget string, content []byte) string {
	// Check symlink target
	switch {
	case strings.Contains(symlinkTarget, "NetworkManager"):
		return "NetworkManager"
	case symlinkTarget == "/run/systemd/resolve/resolv.conf":
		return "systemd-resolved (direct mode, not stub)"
	}

	// Check file header comments (manager annotations appear in the first few lines)
	header := string(content)
	if len(header) > 1024 {
		header = header[:1024]
	}
	headerLower := strings.ToLower(header)

	switch {
	case strings.Contains(headerLower, "tailscale"):
		return "Tailscale"
	case strings.Contains(headerLower, "generated by networkmanager"):
		return "NetworkManager"
	case strings.Contains(headerLower, "generated by resolvconf"):
		return "resolvconf"
	}

	return ""
}

// findStubListenerDisabled scans resolved configuration files for
// DNSStubListener=no. Returns the path of the file where it was found, or "".
func findStubListenerDisabled() string {
	if hasStubListenerDisabled(resolvedConfFile) {
		return resolvedConfFile
	}

	entries, err := os.ReadDir(resolvedConfDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		path := filepath.Join(resolvedConfDir, e.Name())
		if hasStubListenerDisabled(path) {
			return path
		}
	}
	return ""
}

// hasStubListenerDisabled checks if a resolved.conf file contains
// DNSStubListener=no in a [Resolve] section.
func hasStubListenerDisabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	inResolveSection := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") {
			inResolveSection = strings.EqualFold(line, "[Resolve]")
			continue
		}

		if !inResolveSection || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "DNSStubListener") &&
			strings.EqualFold(strings.TrimSpace(val), "no") {
			return true
		}
	}
	return false
}

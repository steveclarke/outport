//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const nssNickname = "Outport Dev CA"

// TrustBrowserCAs adds the CA to Chrome and Firefox NSS databases, and syncs
// Homebrew's cert bundle. Best-effort — returns warning strings for anything
// that didn't work. Never returns an error that would block setup.
func TrustBrowserCAs(certPath string) []string {
	var warnings []string

	if !HasCertutil() {
		warnings = append(warnings, "certutil not found — install "+CertutilInstallHint()+" for browser certificate trust")
	} else {
		warnings = append(warnings, trustNSS(certPath)...)
	}

	syncHomebrewCerts()

	return warnings
}

// UntrustBrowserCAs removes the CA from Chrome and Firefox NSS databases.
// Best-effort — silently ignores failures.
func UntrustBrowserCAs() {
	if !HasCertutil() {
		return
	}
	untrustNSS()
}

// HasCertutil reports whether the certutil binary is on PATH.
func HasCertutil() bool {
	_, err := exec.LookPath("certutil")
	return err == nil
}

// CertutilInstallHint returns a distro-specific package install command for certutil.
func CertutilInstallHint() string {
	hints := []struct {
		binary  string
		command string
	}{
		{"/usr/bin/apt", "libnss3-tools (sudo apt install libnss3-tools)"},
		{"/usr/bin/dnf", "nss-tools (sudo dnf install nss-tools)"},
		{"/usr/bin/pacman", "nss (sudo pacman -S nss)"},
		{"/usr/bin/zypper", "mozilla-nss-tools (sudo zypper install mozilla-nss-tools)"},
	}
	for _, h := range hints {
		if _, err := os.Stat(h.binary); err == nil {
			return h.command
		}
	}
	return "libnss3-tools or nss-tools"
}

// IsNSSTrusted checks if the outport CA is trusted in the given NSS database.
func IsNSSTrusted(dbPath string) bool {
	cmd := exec.Command("certutil", "-L", "-d", "sql:"+dbPath, "-n", nssNickname)
	return cmd.Run() == nil
}

// FindNSSDatabases returns all NSS certificate databases found on the system.
func FindNSSDatabases() []NSSDatabase {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var dbs []NSSDatabase

	// Chrome/Chromium fixed paths
	chromePaths := []struct {
		rel         string
		description string
	}{
		{".pki/nssdb", "Chrome/Chromium"},
		{"snap/chromium/current/.pki/nssdb", "Snap Chromium"},
	}
	for _, cp := range chromePaths {
		dir := filepath.Join(home, cp.rel)
		if hasNSSDB(dir) {
			dbs = append(dbs, NSSDatabase{Path: dir, Description: cp.description})
		}
	}

	// Firefox profile globs
	firefoxGlobs := []struct {
		pattern     string
		description string
	}{
		{filepath.Join(home, ".mozilla/firefox/*"), "Firefox"},
		{filepath.Join(home, "snap/firefox/common/.mozilla/firefox/*"), "Snap Firefox"},
	}
	for _, fg := range firefoxGlobs {
		matches, _ := filepath.Glob(fg.pattern)
		for _, dir := range matches {
			if hasNSSDB(dir) {
				dbs = append(dbs, NSSDatabase{Path: dir, Description: fg.description})
			}
		}
	}

	return dbs
}

// hasNSSDB checks if a directory contains an NSS database (cert9.db).
func hasNSSDB(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "cert9.db"))
	return err == nil
}

// trustNSS adds the CA to all discovered NSS databases.
func trustNSS(certPath string) []string {
	dbs := FindNSSDatabases()
	if len(dbs) == 0 {
		return nil
	}

	var warnings []string
	for _, db := range dbs {
		cmd := exec.Command("certutil", "-A",
			"-d", "sql:"+db.Path,
			"-t", "C,,",
			"-n", nssNickname,
			"-i", certPath,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"failed to add CA to %s (%s): %s",
				db.Description, db.Path, strings.TrimSpace(string(out)),
			))
		}
	}
	return warnings
}

// untrustNSS removes the CA from all discovered NSS databases.
func untrustNSS() {
	for _, db := range FindNSSDatabases() {
		// Ignore errors — cert may not be present in this DB
		_ = exec.Command("certutil", "-D",
			"-d", "sql:"+db.Path,
			"-n", nssNickname,
		).Run()
	}
}

// syncHomebrewCerts re-merges system certs into Homebrew's cert bundle.
// Since Aug 2025, Homebrew's ca-certificates formula includes system CAs
// on postinstall, so this picks up outport's CA after update-ca-certificates.
func syncHomebrewCerts() {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return
	}
	// Only run if ca-certificates is actually installed as a Homebrew formula
	if exec.Command(brewPath, "list", "--formula", "ca-certificates").Run() != nil {
		return
	}
	_ = exec.Command(brewPath, "postinstall", "ca-certificates").Run()
}

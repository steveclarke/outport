//go:build darwin

// Package platform provides OS-specific operations for installing and managing
// the Outport daemon as a system service. On macOS (this file), it handles the
// LaunchAgent plist lifecycle, the /etc/resolver/test DNS configuration file,
// and CA certificate trust via the macOS security framework.
//
// The non-darwin build (other.go) provides stub implementations that return
// "unsupported" errors, keeping the rest of the codebase platform-agnostic.
//
// The macOS integration works as follows: the resolver file tells macOS to send
// all .test DNS queries to 127.0.0.1:15353 (Outport's built-in DNS server).
// The LaunchAgent plist ensures the daemon starts at login and stays running,
// with launchd binding ports 80 and 443 on behalf of the daemon (which avoids
// needing root privileges at runtime). The CA trust step adds Outport's root
// certificate to the login keychain so browsers accept .test HTTPS certificates.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// ResolverPath is the macOS resolver configuration file that directs all .test
	// domain lookups to Outport's local DNS server. macOS checks /etc/resolver/<tld>
	// files to find per-TLD nameserver overrides. This file must be created with sudo
	// because /etc/resolver/ is owned by root.
	ResolverPath = "/etc/resolver/test"

	// ResolverContent is the contents written to the resolver file. It points the
	// .test TLD at 127.0.0.1 on port 15353, which is Outport's built-in DNS server.
	// Port 15353 is used instead of the standard DNS port 53 to avoid conflicts with
	// other DNS services and to allow running without root privileges.
	ResolverContent = "nameserver 127.0.0.1\nport 15353\n"

	// plistName is the filename for the LaunchAgent plist, following Apple's reverse
	// domain naming convention.
	plistName = "dev.outport.daemon.plist"

	// plistLabel is the launchd service label used to identify the daemon in
	// launchctl commands (load, unload, list).
	plistLabel = "dev.outport.daemon"
)

// PlistPath returns the absolute path to the LaunchAgent plist file, located at
// ~/Library/LaunchAgents/dev.outport.daemon.plist. This is the standard location
// for per-user LaunchAgents on macOS. Returns an empty string if the user's home
// directory cannot be determined.
func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func isResolverInstalled() bool {
	_, err := os.Stat(ResolverPath)
	return err == nil
}

func isPlistInstalled() bool {
	_, err := os.Stat(PlistPath())
	return err == nil
}

// WriteResolverFile creates or updates the /etc/resolver/test file, which tells
// macOS to route all .test DNS queries to Outport's local DNS server at
// 127.0.0.1:15353. If the file already exists with the correct content, this is
// a no-op.
//
// This operation requires sudo because /etc/resolver/ is root-owned. The caller
// (typically "outport setup") should inform the user that a password prompt may
// appear. The implementation writes to a temp file first, then uses "sudo cp" to
// place it, avoiding the need to pipe content through sudo.
func WriteResolverFile() error {
	// Skip if file already has the correct content.
	existing, err := os.ReadFile(ResolverPath)
	if err == nil && string(existing) == ResolverContent {
		return nil
	}

	// Ensure /etc/resolver/ exists (not present by default on fresh macOS installs).
	resolverDir := filepath.Dir(ResolverPath)
	mkdirCmd := exec.Command("sudo", "mkdir", "-p", resolverDir)
	mkdirCmd.Stderr = os.Stderr
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("creating resolver directory: %w", err)
	}

	// Write to a temp file, then sudo cp into place.
	tmpFile := fmt.Sprintf("/tmp/outport-resolver-%d", os.Getpid())
	if err := os.WriteFile(tmpFile, []byte(ResolverContent), 0644); err != nil {
		return fmt.Errorf("writing temp resolver file: %w", err)
	}
	defer os.Remove(tmpFile)

	cpCmd := exec.Command("sudo", "cp", tmpFile, ResolverPath)
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copying resolver file: %w", err)
	}
	return nil
}

// RemoveResolverFile deletes /etc/resolver/test, which stops macOS from routing
// .test DNS queries to Outport. This is called during "outport system teardown".
// Requires sudo because the file is root-owned.
func RemoveResolverFile() error {
	cmd := exec.Command("sudo", "rm", "-f", ResolverPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing resolver file: %w", err)
	}
	return nil
}

// WritePlist writes the LaunchAgent plist file that tells macOS launchd how to
// run the Outport daemon. The outportBinary parameter must be the absolute path
// to the outport binary (e.g., /usr/local/bin/outport), which is embedded into
// the plist's ProgramArguments.
//
// The plist configures the daemon to start at login (RunAtLoad), restart if it
// crashes (KeepAlive), and have launchd bind ports 80 and 443 via socket
// activation (Sockets). Socket activation is the mechanism that allows Outport
// to listen on privileged ports without running as root.
func WritePlist(outportBinary string) error {
	content := GeneratePlist(outportBinary)

	path := PlistPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}
	return nil
}

// GeneratePlist returns the plist XML string for the outport daemon LaunchAgent.
// The generated plist configures:
//   - Label: "dev.outport.daemon" (used by launchctl to identify the service).
//   - ProgramArguments: runs "{outportBinary} daemon" to start the daemon process.
//   - RunAtLoad: starts the daemon immediately when the plist is loaded.
//   - KeepAlive: restarts the daemon if it exits unexpectedly.
//   - Sockets: binds HTTP (port 80) and HTTPS (port 443) on 127.0.0.1 via launchd
//     socket activation, so the daemon receives these sockets as file descriptors
//     without needing root privileges.
//   - Logging: stdout and stderr are written to /tmp/outport-daemon.log.
//
// This function is also used in tests to verify plist content without writing to disk.
func GeneratePlist(outportBinary string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>Sockets</key>
    <dict>
        <key>HTTPSocket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>80</string>
        </dict>
        <key>HTTPSSocket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>443</string>
        </dict>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/outport-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/outport-daemon.log</string>
</dict>
</plist>
`, plistLabel, outportBinary)
}

// RemovePlist deletes the LaunchAgent plist file from ~/Library/LaunchAgents/.
// This should be called after UnloadAgent to ensure the daemon is not restarted
// on next login. If the file does not exist, no error is returned.
func RemovePlist() error {
	path := PlistPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}
	return nil
}

// IsAgentLoaded returns true if the Outport LaunchAgent is currently loaded in
// launchd. A loaded agent may or may not be running — it means launchd knows
// about it and will manage its lifecycle. This is checked by running
// "launchctl list dev.outport.daemon", which exits with code 0 if loaded.
func IsAgentLoaded() bool {
	err := exec.Command("launchctl", "list", plistLabel).Run()
	return err == nil
}

// LoadAgent registers and starts the Outport daemon LaunchAgent by running
// "launchctl load" with the plist path. Once loaded, launchd will start the
// daemon immediately (due to RunAtLoad) and restart it if it exits (due to
// KeepAlive). The plist file must already exist on disk (see WritePlist).
func LoadAgent() error {
	cmd := exec.Command("launchctl", "load", PlistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("loading LaunchAgent: %w", err)
	}
	return nil
}

// UnloadAgent stops the running Outport daemon and deregisters its LaunchAgent
// from launchd by running "launchctl unload". After unloading, the daemon will
// not be restarted until LoadAgent is called again (or the user logs out and
// back in, if the plist file still exists).
func UnloadAgent() error {
	cmd := exec.Command("launchctl", "unload", PlistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unloading LaunchAgent: %w", err)
	}
	return nil
}

// TrustCA adds the Outport CA certificate to the macOS login keychain as a
// trusted root certificate. This is what allows browsers (Safari, Chrome, etc.)
// and other TLS clients to accept the .test HTTPS certificates that Outport
// generates. The certPath should point to the CA certificate PEM file (see
// certmanager.CACertPath).
//
// This operation triggers a macOS GUI dialog prompting for the login keychain
// password. If the user cancels the dialog, an error is returned. The certificate
// is added with "trustRoot" policy, meaning it is fully trusted for all purposes.
func TrustCA(certPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}
	keychainPath := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	cmd := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-k", keychainPath, certPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adding CA to trust store (did you cancel the dialog?): %w", err)
	}
	return nil
}

// UntrustCA removes the Outport CA certificate from the macOS trust store. After
// this call, browsers will no longer accept .test HTTPS certificates signed by
// this CA. This is called during teardown to clean up the system trust state.
func UntrustCA(certPath string) error {
	cmd := exec.Command("security", "remove-trusted-cert", certPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing CA from trust store: %w", err)
	}
	return nil
}

// IsCATrusted checks whether the CA certificate at certPath is currently trusted
// by macOS. It runs "security verify-cert" which returns exit code 0 if the
// certificate chain is valid and trusted, or non-zero otherwise. This is used by
// the doctor command to verify that the setup is complete and working.
func IsCATrusted(certPath string) bool {
	err := exec.Command("security", "verify-cert", "-c", certPath).Run()
	return err == nil
}

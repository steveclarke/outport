//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	ResolverPath    = "/etc/resolver/test"
	ResolverContent = "nameserver 127.0.0.1\nport 15353\n"
	plistName       = "dev.outport.daemon.plist"
	plistLabel      = "dev.outport.daemon"
)

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

// WriteResolverFile creates /etc/resolver/test pointing to the local DNS server.
// Requires sudo — the caller should inform the user that a password prompt may appear.
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

// RemoveResolverFile removes /etc/resolver/test.
// Requires sudo.
func RemoveResolverFile() error {
	cmd := exec.Command("sudo", "rm", "-f", ResolverPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing resolver file: %w", err)
	}
	return nil
}

// WritePlist writes the LaunchAgent plist for the outport daemon.
// outportBinary should be the absolute path to the outport binary.
func WritePlist(outportBinary string, httpPort, httpsPort int) error {
	content := GeneratePlist(outportBinary, httpPort, httpsPort)

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

// GeneratePlist returns the plist XML for the outport daemon LaunchAgent.
func GeneratePlist(outportBinary string, httpPort, httpsPort int) string {
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
            <string>%d</string>
        </dict>
        <key>HTTPSSocket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>%d</string>
        </dict>
    </dict>
    <key>StandardOutPath</key>
    <string>/tmp/outport-daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/outport-daemon.log</string>
</dict>
</plist>
`, plistLabel, outportBinary, httpPort, httpsPort)
}

// RemovePlist removes the LaunchAgent plist file.
func RemovePlist() error {
	path := PlistPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}
	return nil
}

// IsAgentLoaded returns true if the LaunchAgent is currently loaded.
func IsAgentLoaded() bool {
	err := exec.Command("launchctl", "list", plistLabel).Run()
	return err == nil
}

// LoadAgent loads the LaunchAgent via launchctl.
func LoadAgent() error {
	cmd := exec.Command("launchctl", "load", PlistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("loading LaunchAgent: %w", err)
	}
	return nil
}

// UnloadAgent unloads the LaunchAgent via launchctl.
func UnloadAgent() error {
	cmd := exec.Command("launchctl", "unload", PlistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unloading LaunchAgent: %w", err)
	}
	return nil
}

// TrustCA adds the CA certificate to the macOS login keychain trust store.
// This triggers a macOS GUI dialog prompting for the login keychain password.
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

// UntrustCA removes the CA certificate from the macOS trust store.
func UntrustCA(certPath string) error {
	cmd := exec.Command("security", "remove-trusted-cert", certPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing CA from trust store: %w", err)
	}
	return nil
}

// IsCATrusted checks if the CA certificate is trusted in the system keychain
// by running "security verify-cert".
func IsCATrusted(certPath string) bool {
	err := exec.Command("security", "verify-cert", "-c", certPath).Run()
	return err == nil
}

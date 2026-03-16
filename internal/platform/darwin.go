//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	resolverPath = "/etc/resolver/test"
	plistName    = "dev.outport.daemon.plist"
	plistLabel   = "dev.outport.daemon"
)

func plistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func isResolverInstalled() bool {
	_, err := os.Stat(resolverPath)
	return err == nil
}

func isPlistInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

// WriteResolverFile creates /etc/resolver/test pointing to the local DNS server.
// Requires sudo — the caller should inform the user that a password prompt may appear.
func WriteResolverFile() error {
	content := "nameserver 127.0.0.1\nport 15353\n"
	cmd := exec.Command("sudo", "tee", resolverPath)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil // suppress tee's stdout echo
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("writing resolver file: %w", err)
	}
	return nil
}

// RemoveResolverFile removes /etc/resolver/test.
// Requires sudo.
func RemoveResolverFile() error {
	cmd := exec.Command("sudo", "rm", "-f", resolverPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing resolver file: %w", err)
	}
	return nil
}

// WritePlist writes the LaunchAgent plist for the outport daemon.
// outportBinary should be the absolute path to the outport binary.
func WritePlist(outportBinary string) error {
	content := GeneratePlist(outportBinary)

	path := plistPath()
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
        <key>Socket</key>
        <dict>
            <key>SockNodeName</key>
            <string>127.0.0.1</string>
            <key>SockServiceName</key>
            <string>80</string>
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

// RemovePlist removes the LaunchAgent plist file.
func RemovePlist() error {
	path := plistPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}
	return nil
}

// LoadAgent loads the LaunchAgent via launchctl.
func LoadAgent() error {
	cmd := exec.Command("launchctl", "load", plistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("loading LaunchAgent: %w", err)
	}
	return nil
}

// UnloadAgent unloads the LaunchAgent via launchctl.
func UnloadAgent() error {
	cmd := exec.Command("launchctl", "unload", plistPath())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unloading LaunchAgent: %w", err)
	}
	return nil
}

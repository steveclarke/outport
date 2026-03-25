//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	serviceName = "outport-daemon"
	serviceFile = serviceName + ".service"

	// ResolverPath is the systemd-resolved drop-in config for .test domains.
	// systemd-resolved (v247+) supports DNS=address:port for per-domain forwarding.
	ResolverPath    = "/etc/systemd/resolved.conf.d/outport-test.conf"
	ResolverContent = "[Resolve]\nDNS=127.0.0.1:15353\nDomains=~test\n"
)

// PlistPath returns the path to the systemd user service file.
// Named PlistPath for API compatibility with darwin.go.
func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "systemd", "user", serviceFile)
}

func isResolverInstalled() bool {
	_, err := os.Stat(ResolverPath)
	return err == nil
}

func isPlistInstalled() bool {
	_, err := os.Stat(PlistPath())
	return err == nil
}

// GeneratePlist returns the systemd service unit content for the outport daemon.
// Named GeneratePlist for API compatibility with darwin.go.
func GeneratePlist(outportBinary string) string {
	return fmt.Sprintf(`[Unit]
Description=Outport development proxy daemon
After=network.target

[Service]
ExecStart=%s daemon
Restart=on-failure

[Install]
WantedBy=default.target
`, outportBinary)
}

// WritePlist writes the systemd user service file and reloads systemd.
func WritePlist(outportBinary string) error {
	content := GeneratePlist(outportBinary)
	path := PlistPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating systemd user directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing service unit: %w", err)
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// RemovePlist removes the systemd user service file and reloads systemd.
func RemovePlist() error {
	path := PlistPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing service unit: %w", err)
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// IsAgentLoaded returns true if the systemd user service is active.
func IsAgentLoaded() bool {
	err := exec.Command("systemctl", "--user", "is-active", "--quiet", serviceName).Run()
	return err == nil
}

// LoadAgent enables and starts the systemd user service.
func LoadAgent() error {
	cmd := exec.Command("systemctl", "--user", "enable", "--now", serviceName)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting outport service: %w", err)
	}
	return nil
}

// UnloadAgent stops the systemd user service.
func UnloadAgent() error {
	cmd := exec.Command("systemctl", "--user", "stop", serviceName)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping outport service: %w", err)
	}
	return nil
}

// WriteResolverFile creates the systemd-resolved drop-in config for .test domains.
// Requires sudo — the caller should inform the user that a password prompt may appear.
func WriteResolverFile() error {
	// Skip if file already has the correct content.
	existing, err := os.ReadFile(ResolverPath)
	if err == nil && string(existing) == ResolverContent {
		return nil
	}

	resolverDir := filepath.Dir(ResolverPath)
	mkdirCmd := exec.Command("sudo", "mkdir", "-p", resolverDir)
	mkdirCmd.Stderr = os.Stderr
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("creating resolved config directory: %w", err)
	}

	tmpFile := fmt.Sprintf("/tmp/outport-resolved-%d", os.Getpid())
	if err := os.WriteFile(tmpFile, []byte(ResolverContent), 0644); err != nil {
		return fmt.Errorf("writing temp resolver config: %w", err)
	}
	defer os.Remove(tmpFile)

	cpCmd := exec.Command("sudo", "cp", tmpFile, ResolverPath)
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copying resolver config: %w", err)
	}

	// Restart systemd-resolved to pick up the new config
	restartCmd := exec.Command("sudo", "systemctl", "restart", "systemd-resolved")
	restartCmd.Stderr = os.Stderr
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("restarting systemd-resolved: %w", err)
	}

	return nil
}

// RemoveResolverFile removes the systemd-resolved drop-in config.
// Requires sudo.
func RemoveResolverFile() error {
	cmd := exec.Command("sudo", "rm", "-f", ResolverPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing resolver config: %w", err)
	}

	// Restart systemd-resolved to drop the .test routing
	restartCmd := exec.Command("sudo", "systemctl", "restart", "systemd-resolved")
	restartCmd.Stderr = os.Stderr
	_ = restartCmd.Run()

	return nil
}

// caTrustConfig describes how a Linux distro handles system CA trust.
type caTrustConfig struct {
	certDir    string
	updateCmd  string
	updateArgs []string
}

// caTrustPaths lists distro-specific CA trust directories and commands.
// Ordered by popularity. Detection works by checking which directory exists.
// This follows the same pattern as mkcert (github.com/FiloSottile/mkcert).
var caTrustPaths = []caTrustConfig{
	{"/usr/local/share/ca-certificates/", "update-ca-certificates", nil},                // Debian/Ubuntu
	{"/etc/pki/ca-trust/source/anchors/", "update-ca-trust", []string{"extract"}},       // Fedora/RHEL
	{"/etc/ca-certificates/trust-source/anchors/", "trust", []string{"extract-compat"}}, // Arch
	{"/usr/share/pki/trust/anchors/", "update-ca-certificates", nil},                    // openSUSE
}

// detectCATrust finds the active CA trust configuration for this distro.
func detectCATrust() (*caTrustConfig, error) {
	for i := range caTrustPaths {
		if _, err := os.Stat(caTrustPaths[i].certDir); err == nil {
			return &caTrustPaths[i], nil
		}
	}
	return nil, fmt.Errorf("could not detect CA trust store — unsupported distro")
}

// TrustCA adds the CA certificate to the system trust store.
// Requires sudo.
func TrustCA(certPath string) error {
	cfg, err := detectCATrust()
	if err != nil {
		return err
	}

	destPath := filepath.Join(cfg.certDir, "outport-ca.crt")
	cpCmd := exec.Command("sudo", "cp", certPath, destPath)
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copying CA to trust store: %w", err)
	}

	args := append([]string{cfg.updateCmd}, cfg.updateArgs...)
	cmd := exec.Command("sudo", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updating CA trust store: %w", err)
	}

	return nil
}

// UntrustCA removes the CA certificate from the system trust store.
// Requires sudo.
func UntrustCA(certPath string) error {
	cfg, err := detectCATrust()
	if err != nil {
		return err
	}

	destPath := filepath.Join(cfg.certDir, "outport-ca.crt")
	rmCmd := exec.Command("sudo", "rm", "-f", destPath)
	rmCmd.Stderr = os.Stderr
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("removing CA from trust store: %w", err)
	}

	args := append([]string{cfg.updateCmd}, cfg.updateArgs...)
	cmd := exec.Command("sudo", args...)
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	return nil
}

// IsCATrusted checks if the outport CA is installed in the system trust store.
func IsCATrusted(certPath string) bool {
	cfg, err := detectCATrust()
	if err != nil {
		return false
	}
	destPath := filepath.Join(cfg.certDir, "outport-ca.crt")
	_, err = os.Stat(destPath)
	return err == nil
}

// EnsurePrivilegedPorts applies CAP_NET_BIND_SERVICE to the outport binary
// so the daemon can bind to ports 80 and 443 without root.
// Requires sudo.
func EnsurePrivilegedPorts(binaryPath string) error {
	cmd := exec.Command("sudo", "setcap", "cap_net_bind_service=+ep", binaryPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setting port capabilities: %w", err)
	}
	return nil
}

// ServiceDescription returns the platform-specific name for the daemon service.
func ServiceDescription() string {
	return "systemd service"
}

// Package settings manages Outport's global user configuration, stored as an INI
// file at ~/.config/outport/config. These settings control daemon-level behavior
// that applies across all projects, such as the health check polling interval and
// DNS TTL values.
//
// The settings file is optional. When it does not exist, Load returns sensible
// defaults. When it does exist, only the keys that are explicitly set override
// their defaults — unset keys keep the built-in default values.
//
// Important design rule: internal packages never import this package directly.
// Instead, the CLI layer (cmd/) calls Load at startup and passes individual
// setting values down as function parameters. This keeps internal packages
// decoupled from the configuration mechanism and easier to test.
package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/ini.v1"
)

// Settings holds all global Outport configuration values. It is the top-level
// struct returned by Load and LoadFrom. Each field is a section-specific struct
// corresponding to a [section] in the INI file.
type Settings struct {
	// Dashboard contains settings that control the web dashboard served at outport.test.
	Dashboard DashboardSettings
	// DNS contains settings that control Outport's built-in DNS server.
	DNS DNSSettings
	// Tunnels contains settings that control the outport share tunnel feature.
	Tunnels TunnelSettings
}

// DashboardSettings controls the behavior of the web dashboard served at outport.test.
type DashboardSettings struct {
	// HealthInterval is how often the dashboard's health checker polls each service
	// to determine if it is running. Health checks only run when at least one browser
	// client is connected via SSE, so this interval has no effect when the dashboard
	// is not open. Set via the "health_interval" key in the [dashboard] section.
	// Default: 3 seconds. Minimum: 1 second.
	HealthInterval time.Duration
}

// TunnelSettings controls the behavior of the `outport share` tunnel feature.
type TunnelSettings struct {
	// Max is the maximum number of concurrent cloudflared tunnel processes.
	// When the cap is reached, primary hostnames are tunneled first, then
	// aliases in config order. Default: 8. Must be greater than 0.
	Max int
}

// DNSSettings controls the behavior of Outport's built-in DNS server, which
// listens on port 15353 and responds to queries for .test hostnames.
type DNSSettings struct {
	// TTL is the time-to-live in seconds included in DNS responses. This tells
	// clients (browsers, curl, etc.) how long they can cache the DNS answer before
	// querying again. Higher values reduce DNS traffic but delay route changes;
	// lower values make changes visible faster. Set via the "ttl" key in the [dns]
	// section. Default: 60 seconds. Must be greater than 0.
	TTL int
}

// Defaults returns a Settings struct populated with the built-in default values
// for all settings. These defaults are used when the config file does not exist
// or when specific keys are omitted from the file. The defaults are:
//   - dashboard.health_interval: 3 seconds
//   - dns.ttl: 60 seconds
//   - tunnels.max: 8
func Defaults() Settings {
	return Settings{
		Dashboard: DashboardSettings{
			HealthInterval: 3 * time.Second,
		},
		DNS: DNSSettings{
			TTL: 60,
		},
		Tunnels: TunnelSettings{
			Max: 8,
		},
	}
}

// Path returns the absolute path to the global settings file, which follows the
// XDG convention: ~/.config/outport/config. This path is used by Load to find
// the settings file and by "outport setup" to create the initial file with
// commented-out defaults.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".config", "outport", "config"), nil
}

// Load reads and parses the global settings file from the default path
// (~/.config/outport/config). If the file does not exist, default settings are
// returned without error. This is the primary entry point used by the daemon at
// startup to obtain configuration values.
func Load() (*Settings, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom reads and parses a settings file from an arbitrary path. This is the
// underlying implementation used by Load, and is also used directly in tests
// where the settings file lives in a temporary directory.
//
// If the file does not exist, default settings are returned without error,
// making the config file entirely optional. If the file exists but contains
// invalid syntax or out-of-range values, an error is returned. Only keys that
// are explicitly present in the file override their defaults.
func LoadFrom(path string) (*Settings, error) {
	s := Defaults()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &s, nil
	}

	cfg, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("parsing settings file: %w", err)
	}

	dashboard := cfg.Section("dashboard")
	if key, err := dashboard.GetKey("health_interval"); err == nil {
		d, err := time.ParseDuration(key.String())
		if err != nil {
			return nil, fmt.Errorf("invalid health_interval: %w", err)
		}
		s.Dashboard.HealthInterval = d
	}

	dns := cfg.Section("dns")
	if key, err := dns.GetKey("ttl"); err == nil {
		v, err := key.Int()
		if err != nil {
			return nil, fmt.Errorf("invalid ttl: %w", err)
		}
		s.DNS.TTL = v
	}

	tunnels := cfg.Section("tunnels")
	if key, err := tunnels.GetKey("max"); err == nil {
		v, err := key.Int()
		if err != nil {
			return nil, fmt.Errorf("invalid tunnels max: %w", err)
		}
		s.Tunnels.Max = v
	}

	if err := s.validate(); err != nil {
		return nil, err
	}

	return &s, nil
}

// validate checks that all settings values are within acceptable ranges.
func (s *Settings) validate() error {
	if s.Dashboard.HealthInterval < time.Second {
		return fmt.Errorf("health_interval %v is too short (minimum 1s)", s.Dashboard.HealthInterval)
	}
	if s.DNS.TTL <= 0 {
		return fmt.Errorf("ttl %d must be greater than 0", s.DNS.TTL)
	}
	if s.Tunnels.Max <= 0 {
		return fmt.Errorf("tunnels max %d must be greater than 0", s.Tunnels.Max)
	}
	return nil
}

// DefaultConfigContent returns the initial contents for a new settings file, with
// all settings present but commented out. This is written by "outport setup" to
// create ~/.config/outport/config, giving users a discoverable reference of
// available settings and their default values without changing any behavior.
func DefaultConfigContent() string {
	return `# Outport global settings
# Uncomment and change values to override defaults.
# Restart the daemon after changes: outport system restart

[dashboard]
# How often the dashboard checks whether services are accepting connections.
# Accepts Go duration syntax: 1s, 5s, 500ms. Minimum 1s.
# health_interval = 3s

[dns]
# Time-to-live in seconds for .test DNS responses. Lower values mean the
# browser picks up service changes faster, but increases DNS queries.
# ttl = 60

[tunnels]
# Maximum number of concurrent tunnel processes for outport share.
# Primary hostnames are tunneled first, then aliases.
# max = 8
`
}

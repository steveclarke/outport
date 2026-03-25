package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/ini.v1"
)

// Settings holds all global Outport configuration.
type Settings struct {
	Dashboard DashboardSettings
	DNS       DNSSettings
}

// DashboardSettings controls dashboard behaviour.
type DashboardSettings struct {
	HealthInterval time.Duration
}

// DNSSettings controls DNS behaviour.
type DNSSettings struct {
	TTL int
}

// Defaults returns a Settings with the built-in default values.
func Defaults() Settings {
	return Settings{
		Dashboard: DashboardSettings{
			HealthInterval: 3 * time.Second,
		},
		DNS: DNSSettings{
			TTL: 60,
		},
	}
}

// Path returns the default path for the global settings file:
// ~/.config/outport/config
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".config", "outport", "config"), nil
}

// Load loads settings from the default path.
func Load() (*Settings, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom loads settings from the given path. If the file does not exist,
// default settings are returned without error.
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
	return nil
}

// DefaultConfigContent returns the commented-out default config file contents.
func DefaultConfigContent() string {
	return `# Outport global settings
# Uncomment and change values to override defaults.

[dashboard]
# health_interval = 3s

[dns]
# ttl = 60
`
}

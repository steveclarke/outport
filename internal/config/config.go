package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = ".outport.yml"

type Service struct {
	DefaultPort int    `yaml:"default_port"`
	EnvVar      string `yaml:"env_var"`
}

type Config struct {
	Name     string             `yaml:"name"`
	Services map[string]Service `yaml:"services"`
}

func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("config: 'name' is required")
	}
	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("config: at least one service is required")
	}

	return &cfg, nil
}

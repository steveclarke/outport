package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const FileName = ".outport.yml"

// EnvFileField handles YAML that can be a string or []string.
type EnvFileField []string

func (e *EnvFileField) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*e = []string{value.Value}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*e = list
		return nil
	}
	return fmt.Errorf("env_file must be a string or list of strings")
}

type Service struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Protocol      string       `yaml:"protocol"`
	RawEnvFile    EnvFileField `yaml:"env_file"`
	EnvFiles      []string     `yaml:"-"`
	Group         string       `yaml:"-"`
}

type Group struct {
	EnvFile  string             `yaml:"env_file"`
	Services map[string]Service `yaml:"services"`
}

type Config struct {
	Name     string             `yaml:"name"`
	Services map[string]Service `yaml:"services"`
	Groups   map[string]Group   `yaml:"groups"`
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

	if err := cfg.normalize(); err != nil {
		return nil, err
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("config: at least one service is required")
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) normalize() error {
	if c.Services == nil {
		c.Services = make(map[string]Service)
	}

	// Validate groups are not empty
	for groupName, group := range c.Groups {
		if len(group.Services) == 0 {
			return fmt.Errorf("config: group %q has no services", groupName)
		}
	}

	// Flatten group services into top-level Services
	for groupName, group := range c.Groups {
		for svcName, svc := range group.Services {
			if _, exists := c.Services[svcName]; exists {
				return fmt.Errorf("config: duplicate service name %q", svcName)
			}
			if len(svc.RawEnvFile) == 0 && group.EnvFile != "" {
				svc.RawEnvFile = EnvFileField{group.EnvFile}
			}
			svc.Group = groupName
			c.Services[svcName] = svc
		}
	}

	// Resolve defaults for all services
	for name, svc := range c.Services {
		if len(svc.RawEnvFile) == 0 {
			svc.EnvFiles = []string{".env"}
		} else {
			svc.EnvFiles = []string(svc.RawEnvFile)
		}
		c.Services[name] = svc
	}

	return nil
}

func (c *Config) validate() error {
	fileVars := make(map[string]map[string]string)

	for name, svc := range c.Services {
		if svc.EnvVar == "" {
			return fmt.Errorf("config: service %q is missing required field 'env_var'", name)
		}
		for _, envFile := range svc.EnvFiles {
			if fileVars[envFile] == nil {
				fileVars[envFile] = make(map[string]string)
			}
			if other, exists := fileVars[envFile][svc.EnvVar]; exists {
				return fmt.Errorf("config: services %q and %q both write %s to %s",
					other, name, svc.EnvVar, envFile)
			}
			fileVars[envFile][svc.EnvVar] = name
		}
	}

	return nil
}

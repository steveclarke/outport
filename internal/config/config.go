package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const FileName = ".outport.yml"

// envFileField handles YAML that can be a string or []string.
type envFileField []string

func (e *envFileField) UnmarshalYAML(value *yaml.Node) error {
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
	rawEnvFile    envFileField // populated during YAML unmarshal, resolved to EnvFiles in normalize
	EnvFiles      []string     `yaml:"-"`
	Group         string       `yaml:"-"`
}

// rawService is used for YAML unmarshaling to capture env_file before normalization.
type rawService struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Protocol      string       `yaml:"protocol"`
	EnvFile       envFileField `yaml:"env_file"`
}

type Group struct {
	EnvFile     string                `yaml:"env_file"`
	RawServices map[string]rawService `yaml:"services"`
}

// rawConfig is the YAML deserialization target.
type rawConfig struct {
	Name        string                `yaml:"name"`
	RawServices map[string]rawService `yaml:"services"`
	Groups      map[string]Group      `yaml:"groups"`
}

type Config struct {
	Name     string
	Services map[string]Service
}

func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("No %s found in %s. Run 'outport init' to create one.", FileName, dir)
		}
		return nil, fmt.Errorf("Could not read %s: %w.", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("Invalid YAML in %s: %w.", FileName, err)
	}

	if raw.Name == "" {
		return nil, fmt.Errorf("The 'name' field is missing in %s.", FileName)
	}

	cfg := &Config{
		Name:     raw.Name,
		Services: make(map[string]Service),
	}

	if err := cfg.normalize(&raw); err != nil {
		return nil, err
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("No services defined in %s. Add services or groups.", FileName)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func toService(rs rawService) Service {
	return Service{
		PreferredPort: rs.PreferredPort,
		EnvVar:        rs.EnvVar,
		Protocol:      rs.Protocol,
		rawEnvFile:    rs.EnvFile,
	}
}

func (c *Config) normalize(raw *rawConfig) error {
	// Add top-level services
	for name, rs := range raw.RawServices {
		c.Services[name] = toService(rs)
	}

	// Sort group names for deterministic error messages
	groupNames := make([]string, 0, len(raw.Groups))
	for name := range raw.Groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	// Validate groups are not empty, then flatten
	for _, groupName := range groupNames {
		group := raw.Groups[groupName]
		if len(group.RawServices) == 0 {
			return fmt.Errorf("Group %q in %s has no services.", groupName, FileName)
		}
		for svcName, rs := range group.RawServices {
			if _, exists := c.Services[svcName]; exists {
				return fmt.Errorf("Duplicate service name %q in %s. Service names must be unique across all groups.", svcName, FileName)
			}
			svc := toService(rs)
			if len(svc.rawEnvFile) == 0 && group.EnvFile != "" {
				svc.rawEnvFile = envFileField{group.EnvFile}
			}
			svc.Group = groupName
			c.Services[svcName] = svc
		}
	}

	// Resolve env_file defaults for all services
	for name, svc := range c.Services {
		if len(svc.rawEnvFile) == 0 {
			svc.EnvFiles = []string{".env"}
		} else {
			svc.EnvFiles = []string(svc.rawEnvFile)
		}
		svc.rawEnvFile = nil // clear intermediate state
		c.Services[name] = svc
	}

	return nil
}

func (c *Config) validate() error {
	fileVars := make(map[string]map[string]string)

	for name, svc := range c.Services {
		if svc.EnvVar == "" {
			return fmt.Errorf("Service %q in %s is missing the 'env_var' field.", name, FileName)
		}
		for _, envFile := range svc.EnvFiles {
			if fileVars[envFile] == nil {
				fileVars[envFile] = make(map[string]string)
			}
			if other, exists := fileVars[envFile][svc.EnvVar]; exists {
				return fmt.Errorf("Services %q and %q both write %s to %s. Each env var must be unique per file.",
					other, name, svc.EnvVar, envFile)
			}
			fileVars[envFile][svc.EnvVar] = name
		}
	}

	return nil
}

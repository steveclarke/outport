package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var templateVarRe = regexp.MustCompile(`\$\{(\w+)\}`)

// ResolveDerived substitutes ${VAR_NAME} references in derived values
// with the corresponding port numbers from envVarPorts.
func ResolveDerived(derived map[string]DerivedValue, envVarPorts map[string]int) map[string]string {
	resolved := make(map[string]string)
	for name, dv := range derived {
		value := dv.Value
		for varName, port := range envVarPorts {
			value = strings.ReplaceAll(value, "${"+varName+"}", strconv.Itoa(port))
		}
		resolved[name] = value
	}
	return resolved
}

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
}

// rawService is used for YAML unmarshaling to capture env_file before normalization.
type rawService struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Protocol      string       `yaml:"protocol"`
	EnvFile       envFileField `yaml:"env_file"`
}

type DerivedValue struct {
	Value    string   `yaml:"value"`
	EnvFiles []string `yaml:"-"`
}

type rawDerivedValue struct {
	Value   string       `yaml:"value"`
	EnvFile envFileField `yaml:"env_file"`
}

// rawConfig is the YAML deserialization target.
type rawConfig struct {
	Name        string                     `yaml:"name"`
	RawServices map[string]rawService      `yaml:"services"`
	RawDerived  map[string]rawDerivedValue `yaml:"derived"`
}

type Config struct {
	Name     string
	Services map[string]Service
	Derived  map[string]DerivedValue
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
		Derived:  make(map[string]DerivedValue),
	}

	if err := cfg.normalize(&raw); err != nil {
		return nil, err
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("No services defined in %s.", FileName)
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
	for name, rs := range raw.RawServices {
		svc := toService(rs)
		if len(svc.rawEnvFile) == 0 {
			svc.EnvFiles = []string{".env"}
		} else {
			svc.EnvFiles = []string(svc.rawEnvFile)
		}
		svc.rawEnvFile = nil
		c.Services[name] = svc
	}

	for name, rd := range raw.RawDerived {
		dv := DerivedValue{
			Value:    rd.Value,
			EnvFiles: []string(rd.EnvFile),
		}
		c.Derived[name] = dv
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

	// Build set of valid env_var names from services
	serviceEnvVars := make(map[string]bool)
	for _, svc := range c.Services {
		serviceEnvVars[svc.EnvVar] = true
	}

	for name, dv := range c.Derived {
		if dv.Value == "" {
			return fmt.Errorf("Derived value %q in %s is missing the 'value' field.", name, FileName)
		}
		if len(dv.EnvFiles) == 0 {
			return fmt.Errorf("Derived value %q in %s is missing the 'env_file' field.", name, FileName)
		}

		// Name must not collide with any service env_var
		if serviceEnvVars[name] {
			return fmt.Errorf("Derived value %q in %s conflicts with a service env_var of the same name.", name, FileName)
		}

		// All ${VAR} references must match a service env_var
		matches := templateVarRe.FindAllStringSubmatch(dv.Value, -1)
		for _, match := range matches {
			ref := match[1]
			if !serviceEnvVars[ref] {
				return fmt.Errorf("Derived value %q in %s references \"${%s}\" but no service has env_var %q.",
					name, FileName, ref, ref)
			}
		}
	}

	return nil
}

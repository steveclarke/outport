package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// hostnameRe validates hostname stems: lowercase alphanumeric, hyphens, and dots.
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// templateVarRe matches ${service.field} or ${service.field:modifier} references in derived value templates.
var templateVarRe = regexp.MustCompile(`\$\{(\w+)\.(\w+)(?::(\w+))?\}`)

// validFields are the service fields that can be referenced in templates.
var validFields = map[string]bool{
	"port":     true,
	"hostname": true,
	"url":      true,
}

// validModifiers maps field names to their allowed modifiers.
var validModifiers = map[string]map[string]bool{
	"url": {"direct": true},
}

func validateTemplateRefs(derivedName, template string, services map[string]Service) error {
	if template == "" {
		return nil
	}
	matches := templateVarRe.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		svcName := m[1]
		field := m[2]
		modifier := ""
		if len(m) > 3 {
			modifier = m[3]
		}

		if _, ok := services[svcName]; !ok {
			return fmt.Errorf("derived %q: references unknown service %q", derivedName, svcName)
		}
		if !validFields[field] {
			return fmt.Errorf("derived %q: unknown field %q (valid: port, hostname, url)", derivedName, field)
		}
		if modifier != "" {
			mods, ok := validModifiers[field]
			if !ok || !mods[modifier] {
				return fmt.Errorf("derived %q: unknown modifier %q for field %q", derivedName, modifier, field)
			}
		}
	}
	return nil
}

// ResolveDerived substitutes ${service.field} references in derived values
// with the corresponding values from templateVars.
// Returns name → file → resolved value.
func ResolveDerived(derived map[string]DerivedValue, templateVars map[string]string) map[string]map[string]string {
	resolved := make(map[string]map[string]string)
	for name, dv := range derived {
		fileValues := make(map[string]string)
		for _, file := range dv.EnvFiles {
			template := dv.Value
			if pf, ok := dv.PerFile[file]; ok {
				template = pf
			}
			value := template
			for varName, varValue := range templateVars {
				value = strings.ReplaceAll(value, "${"+varName+"}", varValue)
			}
			fileValues[file] = value
		}
		resolved[name] = fileValues
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
	Hostname      string       `yaml:"hostname"`
	rawEnvFile    envFileField // populated during YAML unmarshal, resolved to EnvFiles in normalize
	EnvFiles      []string     `yaml:"-"`
}

// rawService is used for YAML unmarshaling to capture env_file before normalization.
type rawService struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Protocol      string       `yaml:"protocol"`
	Hostname      string       `yaml:"hostname"`
	EnvFile       envFileField `yaml:"env_file"`
}

type DerivedValue struct {
	Value    string            `yaml:"value"`
	EnvFiles []string          `yaml:"-"`
	PerFile  map[string]string `yaml:"-"` // file → value template (overrides Value)
}

// derivedEnvFileEntry is a single entry in a derived value's env_file list.
// Can be a plain string or an object with file + value.
type derivedEnvFileEntry struct {
	File  string `yaml:"file"`
	Value string `yaml:"value"`
}

// derivedEnvFileField handles YAML that can be a string, []string, or []object.
type derivedEnvFileField []derivedEnvFileEntry

func (d *derivedEnvFileField) UnmarshalYAML(value *yaml.Node) error {
	// Single string: "frontend/.env"
	if value.Kind == yaml.ScalarNode {
		*d = []derivedEnvFileEntry{{File: value.Value}}
		return nil
	}
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("env_file must be a string or list")
	}
	// List — each item can be a string or an object with file + value
	for _, item := range value.Content {
		if item.Kind == yaml.ScalarNode {
			*d = append(*d, derivedEnvFileEntry{File: item.Value})
		} else if item.Kind == yaml.MappingNode {
			var entry derivedEnvFileEntry
			if err := item.Decode(&entry); err != nil {
				return fmt.Errorf("invalid env_file entry: %w", err)
			}
			*d = append(*d, entry)
		} else {
			return fmt.Errorf("env_file entries must be strings or objects with file + value")
		}
	}
	return nil
}

type rawDerivedValue struct {
	Value   string              `yaml:"value"`
	EnvFile derivedEnvFileField `yaml:"env_file"`
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

// FindDir walks up from startDir looking for .outport.yml.
// Returns the directory containing the config file.
func FindDir(startDir string) (string, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, FileName)
		if _, err := os.Stat(path); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("No %s found in %s or any parent directory. Run 'outport init' to create one.", FileName, startDir)
		}
		dir = parent
	}
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
		Hostname:      rs.Hostname,
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
			Value:   rd.Value,
			PerFile: make(map[string]string),
		}
		for _, entry := range rd.EnvFile {
			dv.EnvFiles = append(dv.EnvFiles, entry.File)
			if entry.Value != "" {
				dv.PerFile[entry.File] = entry.Value
			}
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

	for name, svc := range c.Services {
		if svc.Hostname != "" {
			if svc.Protocol != "http" && svc.Protocol != "https" {
				return fmt.Errorf("service %q: hostname requires protocol http or https", name)
			}
			if !hostnameRe.MatchString(svc.Hostname) {
				return fmt.Errorf("service %q: hostname %q contains invalid characters (use lowercase alphanumeric, hyphens, dots)", name, svc.Hostname)
			}
			if !strings.Contains(svc.Hostname, c.Name) {
				return fmt.Errorf("service %q: hostname %q must contain project name %q", name, svc.Hostname, c.Name)
			}
		}
	}

	// Build set of valid env_var names from services
	serviceEnvVars := make(map[string]bool)
	for _, svc := range c.Services {
		serviceEnvVars[svc.EnvVar] = true
	}

	for name, dv := range c.Derived {
		if len(dv.EnvFiles) == 0 {
			return fmt.Errorf("Derived value %q in %s is missing the 'env_file' field.", name, FileName)
		}

		// Check if any env_file entries need the top-level value
		for _, file := range dv.EnvFiles {
			if _, hasPerFile := dv.PerFile[file]; !hasPerFile && dv.Value == "" {
				return fmt.Errorf("Derived value %q in %s is missing the 'value' field (required for entries without per-file values).", name, FileName)
			}
		}

		// Name must not collide with any service env_var
		if serviceEnvVars[name] {
			return fmt.Errorf("Derived value %q in %s conflicts with a service env_var of the same name.", name, FileName)
		}

		// Validate ${service.field} references in top-level value
		if err := validateTemplateRefs(name, dv.Value, c.Services); err != nil {
			return err
		}

		// Validate references in per-file values
		for _, pfValue := range dv.PerFile {
			if err := validateTemplateRefs(name, pfValue, c.Services); err != nil {
				return err
			}
		}
	}

	return nil
}

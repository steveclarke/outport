// Package config loads and validates the outport.yml project configuration file.
//
// Each project that uses Outport has an outport.yml in its root directory. This file
// declares the project's services (with their port preferences, environment variables,
// and optional hostnames) and any computed values (derived environment variables built
// from templates that reference service fields).
//
// The package handles YAML deserialization, default values, template reference validation,
// and normalization of flexible YAML syntax (e.g., env_file can be a string or a list).
// It does not perform port allocation or registry operations -- those are handled by
// the allocator and registry packages respectively.
//
// Typical usage from a CLI command:
//
//	dir, err := config.FindDir(startDir)  // walk up to find outport.yml
//	cfg, err := config.Load(dir)          // parse, normalize, and validate
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

// templateVarRe matches ${service.field} or ${service.field:modifier} references in computed value templates.
var templateVarRe = regexp.MustCompile(`\$\{(\w+)\.(\w+)(?::(\w+))?\}`)

// standaloneVarRe matches ${word} references that don't contain a dot (i.e., not service.field).
// It also matches ${word:-...} and ${word:+...} conditional syntax.
var standaloneVarRe = regexp.MustCompile(`\$\{(\w+)\}|\$\{(\w+):[+-]`)

// validFields are the service fields that can be referenced in templates.
var validFields = map[string]bool{
	"port":     true,
	"hostname": true,
	"url":      true,
	"env_var":  true,
}

// validModifiers maps field names to their allowed modifiers.
var validModifiers = map[string]map[string]bool{
	"url": {"direct": true},
}

// validStandaloneVars are top-level template variables (not service-scoped).
var validStandaloneVars = map[string]bool{
	"instance":     true,
	"project_name": true,
}

func validateTemplateRefs(computedName, template string, services map[string]Service) error {
	if template == "" {
		return nil
	}

	// Validate ${service.field} and ${service.field:modifier} references
	matches := templateVarRe.FindAllStringSubmatch(template, -1)
	for _, m := range matches {
		svcName := m[1]
		field := m[2]
		modifier := ""
		if len(m) > 3 {
			modifier = m[3]
		}

		if _, ok := services[svcName]; !ok {
			return fmt.Errorf("computed %q: references unknown service %q", computedName, svcName)
		}
		if !validFields[field] {
			return fmt.Errorf("computed %q: unknown field %q (valid: port, hostname, url, env_var)", computedName, field)
		}
		if modifier != "" {
			mods, ok := validModifiers[field]
			if !ok || !mods[modifier] {
				return fmt.Errorf("computed %q: unknown modifier %q for field %q", computedName, modifier, field)
			}
		}
	}

	// Validate standalone ${var} and ${var:-...} / ${var:+...} references
	standaloneMatches := standaloneVarRe.FindAllStringSubmatch(template, -1)
	for _, m := range standaloneMatches {
		varName := m[1]
		if varName == "" {
			varName = m[2] // from the ${word:[+-] branch
		}
		if !validStandaloneVars[varName] {
			return fmt.Errorf("computed %q: unknown variable %q (valid: instance, project_name)", computedName, varName)
		}
	}

	return nil
}

// ResolveComputed substitutes template variable references in all computed values,
// producing the final environment variable values ready to be written to env files.
//
// Each computed value may have a default template (Value) and optional per-file
// overrides (PerFile). For each env file a computed value targets, this function
// selects the appropriate template and expands it using ExpandVars with the
// provided templateVars map (which contains entries like "rails.port" = "10042",
// "rails.hostname" = "myapp.test", "instance" = "xbjf", etc.).
//
// The return value is a nested map: computed variable name -> env file path -> resolved value.
// For example: {"DATABASE_URL": {".env": "postgres://localhost:10042/myapp"}}.
func ResolveComputed(computed map[string]ComputedValue, templateVars map[string]string) map[string]map[string]string {
	resolved := make(map[string]map[string]string)
	for name, dv := range computed {
		fileValues := make(map[string]string)
		for _, file := range dv.EnvFiles {
			template := dv.Value
			if pf, ok := dv.PerFile[file]; ok {
				template = pf
			}
			value := ExpandVars(template, templateVars)
			fileValues[file] = value
		}
		resolved[name] = fileValues
	}
	return resolved
}

// FileName is the name of the Outport configuration file that must exist in a
// project's root directory. CLI commands use FindDir to locate this file by
// walking up from the current working directory.
const FileName = "outport.yml"

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

// Service represents a single service declared in the project's outport.yml file.
// Each service gets a deterministic port allocation and has its port written to one
// or more .env files as the specified environment variable.
//
// For example, a "rails" service might get port 10042 written as PORT=10042 to .env,
// and optionally be accessible at myapp.test via the local proxy.
type Service struct {
	// PreferredPort is an optional hint for the port allocator. When set, the allocator
	// will try to assign this exact port. If it collides with another service, the
	// allocator falls back to its hash-based algorithm. Zero means no preference.
	PreferredPort int `yaml:"preferred_port"`

	// EnvVar is the environment variable name that will hold this service's allocated port
	// (e.g., "PORT", "VITE_PORT"). This is written to the service's env files inside a
	// fenced block. Required -- validation rejects services without it.
	EnvVar string `yaml:"env_var"`

	// Hostname is the optional .test domain hostname for this service (e.g., "myapp.test").
	// When set, the daemon's DNS server and HTTP/TLS proxy will route requests for this
	// hostname to the service's allocated port. Must contain the project name and use only
	// lowercase alphanumeric characters, hyphens, and dots. "outport.test" is reserved.
	Hostname string `yaml:"hostname"`

	// rawEnvFile holds the YAML-deserialized env_file value before normalization.
	// It is cleared during normalize and should not be accessed after Load returns.
	rawEnvFile envFileField

	// EnvFiles is the resolved list of env file paths where this service's port variable
	// will be written. Defaults to [".env"] if not specified in the YAML. Paths are
	// relative to the project root directory.
	EnvFiles []string `yaml:"-"`
}

// rawService is used for YAML unmarshaling to capture env_file before normalization.
type rawService struct {
	PreferredPort int          `yaml:"preferred_port"`
	EnvVar        string       `yaml:"env_var"`
	Hostname      string       `yaml:"hostname"`
	EnvFile       envFileField `yaml:"env_file"`
}

// ComputedValue represents a derived environment variable whose value is built from a
// template that references other services' fields. Computed values let projects define
// compound variables like DATABASE_URL that combine a service's port and hostname.
//
// Templates use bash-style parameter expansion syntax. Service fields are referenced as
// ${service.field} (e.g., "${rails.port}", "${web.url}") and standalone variables as
// ${var} (e.g., "${instance}", "${project_name}"). Conditional syntax like ${var:-default}
// and ${var:+replacement} is also supported. See ExpandVars for full details.
type ComputedValue struct {
	// Value is the default template string used for all env files unless overridden
	// by a per-file entry. For example: "postgres://localhost:${db.port}/${project_name}".
	// May be empty if every env file has a per-file override in PerFile.
	Value string `yaml:"value"`

	// EnvFiles is the list of env file paths where this computed variable will be written.
	// Unlike Service.EnvFiles, there is no default -- at least one file must be specified.
	EnvFiles []string `yaml:"-"`

	// PerFile maps env file paths to file-specific template overrides. When a file appears
	// in this map, its template is used instead of Value. This allows the same computed
	// variable to have different formats in different env files (e.g., a URL with a proxy
	// hostname for one file and a direct localhost URL for another).
	PerFile map[string]string `yaml:"-"`
}

// computedEnvFileEntry is a single entry in a computed value's env_file list.
// Can be a plain string or an object with file + value.
type computedEnvFileEntry struct {
	File  string `yaml:"file"`
	Value string `yaml:"value"`
}

// computedEnvFileField handles YAML that can be a string, []string, or []object.
type computedEnvFileField []computedEnvFileEntry

func (d *computedEnvFileField) UnmarshalYAML(value *yaml.Node) error {
	// Single string: "frontend/.env"
	if value.Kind == yaml.ScalarNode {
		*d = []computedEnvFileEntry{{File: value.Value}}
		return nil
	}
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("env_file must be a string or list")
	}
	// List — each item can be a string or an object with file + value
	for _, item := range value.Content {
		if item.Kind == yaml.ScalarNode {
			*d = append(*d, computedEnvFileEntry{File: item.Value})
		} else if item.Kind == yaml.MappingNode {
			var entry computedEnvFileEntry
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

type rawComputedValue struct {
	Value   string               `yaml:"value"`
	EnvFile computedEnvFileField `yaml:"env_file"`
}

// rawConfig is the YAML deserialization target.
type rawConfig struct {
	Name        string                      `yaml:"name"`
	RawServices map[string]rawService       `yaml:"services"`
	RawComputed map[string]rawComputedValue `yaml:"computed"`
}

// Config is the fully parsed, normalized, and validated representation of a project's
// outport.yml file. It is the primary data structure that CLI commands and the allocation
// package use to understand what a project needs from Outport.
//
// Config is always created via Load, which handles YAML deserialization, default values,
// and validation. It should be treated as read-only after construction.
type Config struct {
	// Name is the project identifier from the "name" field in outport.yml. It is used
	// as part of the hash key for deterministic port allocation ("{project}/{instance}/{service}")
	// and must be present in any service hostnames. Required -- Load rejects configs without it.
	Name string

	// Services maps service names (e.g., "rails", "vite", "sidekiq") to their configuration.
	// At least one service must be defined. Service names are the keys from the "services"
	// map in outport.yml and are used in the port allocation hash and in template references.
	Services map[string]Service

	// Computed maps environment variable names (e.g., "DATABASE_URL") to their computed value
	// definitions. Computed values are optional. Their names must not collide with any
	// service's EnvVar.
	Computed map[string]ComputedValue
}

// FindDir walks up the directory tree from startDir looking for an outport.yml file.
// It returns the absolute path of the directory containing the config file.
//
// This is the standard way CLI commands locate the project root. For example, if the
// user runs "outport status" from /home/user/myapp/app/models, FindDir will check
// each parent directory until it finds /home/user/myapp/outport.yml and return
// "/home/user/myapp".
//
// Returns an error with setup instructions if no config file is found in any
// ancestor directory.
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

// Load reads, parses, normalizes, and validates the outport.yml file in the given directory.
// It returns a fully populated Config ready for use by the allocation package and CLI commands.
//
// The loading process has four stages:
//  1. Read and parse the YAML file into raw deserialization types.
//  2. Normalize the raw data: resolve flexible YAML syntax (e.g., string-or-list env_file
//     fields), apply defaults (services without env_file get [".env"]), and build the
//     final Service and ComputedValue maps.
//  3. Validate the config: ensure required fields are present (name, env_var), check that
//     hostnames follow naming rules and contain the project name, verify that computed
//     value template references point to real services and valid fields, and detect
//     env_var name collisions within the same env file.
//  4. Return the validated Config.
//
// Returns a descriptive error if any stage fails, with messages designed to guide the
// user toward a fix.
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
		Computed: make(map[string]ComputedValue),
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

	for name, rd := range raw.RawComputed {
		dv := ComputedValue{
			Value:   rd.Value,
			PerFile: make(map[string]string),
		}
		for _, entry := range rd.EnvFile {
			dv.EnvFiles = append(dv.EnvFiles, entry.File)
			if entry.Value != "" {
				dv.PerFile[entry.File] = entry.Value
			}
		}
		c.Computed[name] = dv
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
			if svc.Hostname == "outport.test" {
				return fmt.Errorf("service %q: hostname %q is reserved for the Outport dashboard", name, svc.Hostname)
			}
			stem := strings.TrimSuffix(svc.Hostname, ".test")
			if !hostnameRe.MatchString(stem) {
				return fmt.Errorf("service %q: hostname %q contains invalid characters (use lowercase alphanumeric, hyphens, dots)", name, svc.Hostname)
			}
			if !strings.Contains(stem, c.Name) {
				return fmt.Errorf("service %q: hostname %q must contain project name %q", name, svc.Hostname, c.Name)
			}
		}
	}

	// Build set of valid env_var names from services
	serviceEnvVars := make(map[string]bool)
	for _, svc := range c.Services {
		serviceEnvVars[svc.EnvVar] = true
	}

	for name, dv := range c.Computed {
		if len(dv.EnvFiles) == 0 {
			return fmt.Errorf("Computed value %q in %s is missing the 'env_file' field.", name, FileName)
		}

		// Check if any env_file entries need the top-level value
		for _, file := range dv.EnvFiles {
			if _, hasPerFile := dv.PerFile[file]; !hasPerFile && dv.Value == "" {
				return fmt.Errorf("Computed value %q in %s is missing the 'value' field (required for entries without per-file values).", name, FileName)
			}
		}

		// Name must not collide with any service env_var
		if serviceEnvVars[name] {
			return fmt.Errorf("Computed value %q in %s conflicts with a service env_var of the same name.", name, FileName)
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

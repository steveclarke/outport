// Package allocation builds registry allocations from a project's config and its
// assigned ports. It is the bridge between raw port numbers (from the allocator
// package) and the rich metadata stored in the registry.
//
// Responsibilities include:
//   - Computing .test hostnames for services, with instance-aware suffixing so
//     that worktree checkouts get unique hostnames (e.g., myapp-bxcf.test).
//   - Extracting env_var declarations from service configs.
//   - Building the template variable map (${service.port}, ${service.hostname},
//     ${service.url}, etc.) used for bash-style parameter expansion in computed values.
//   - Resolving computed value templates into their final strings per env file.
//
// This package contains pure domain logic with no CLI or I/O dependencies.
package allocation

import (
	"fmt"
	"strings"

	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/urlutil"
)

// Build constructs a complete registry.Allocation from a project's config, its
// instance name, the project directory path, and the map of service-name-to-port
// assignments. It assembles all the pieces that the registry needs to persist:
// the project directory, port map, computed hostnames, and env var declarations.
//
// This is the primary entry point for creating allocations during `outport up`.
// The returned Allocation is ready to be saved to the registry via registry.Set.
func Build(cfg *config.Config, instanceName, dir string, ports map[string]int) registry.Allocation {
	return registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
		Hostnames:  ComputeHostnames(cfg, instanceName),
		Aliases:    ComputeAliases(cfg, instanceName),
		EnvVars:    computeEnvVars(cfg),
	}
}

// ComputeHostnames builds a map of service name to .test hostname for every service
// in the config that declares a hostname field.
//
// For the "main" instance (the first checkout of a project), hostnames are used
// as-is from the config with a .test suffix. For example, a service with
// hostname "myapp" becomes "myapp.test".
//
// For non-main instances (worktrees and additional clones), the project name
// portion of the hostname stem is suffixed with the instance code to ensure
// global uniqueness. For example, if the project name is "myapp" and the instance
// code is "bxcf", hostname "myapp" becomes "myapp-bxcf.test", and a compound
// hostname like "api-myapp" becomes "api-myapp-bxcf.test".
//
// Services that do not declare a hostname in their config are omitted from the
// returned map.
func ComputeHostnames(cfg *config.Config, instanceName string) map[string]string {
	hostnames := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc.Hostname == "" {
			continue
		}
		stem := strings.TrimSuffix(svc.Hostname, ".test")
		if instanceName != "main" {
			idx := strings.LastIndex(stem, cfg.Name)
			if idx >= 0 {
				stem = stem[:idx] + cfg.Name + "-" + instanceName + stem[idx+len(cfg.Name):]
			}
		}
		hostnames[name] = stem + ".test"
	}
	return hostnames
}

// ComputeAliases builds a map of service name -> alias name -> .test hostname
// for every service that declares aliases in the config. Instance suffixing
// follows the same rules as primary hostnames.
func ComputeAliases(cfg *config.Config, instanceName string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for svcName, svc := range cfg.Services {
		if len(svc.Aliases) == 0 {
			continue
		}
		svcAliases := make(map[string]string)
		for key, aliasHostname := range svc.Aliases {
			stem := strings.TrimSuffix(aliasHostname, ".test")
			if instanceName != "main" {
				idx := strings.LastIndex(stem, cfg.Name)
				if idx >= 0 {
					stem = stem[:idx] + cfg.Name + "-" + instanceName + stem[idx+len(cfg.Name):]
				}
			}
			svcAliases[key] = stem + ".test"
		}
		result[svcName] = svcAliases
	}
	return result
}

// computeEnvVars builds a service name -> env_var map from config.
func computeEnvVars(cfg *config.Config) map[string]string {
	envVars := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc.EnvVar != "" {
			envVars[name] = svc.EnvVar
		}
	}
	return envVars
}

// BuildTemplateVars builds the template variable map used for bash-style parameter
// expansion in computed values and other template contexts. The returned map uses
// dotted keys like "service.field" that correspond to the ${service.field} syntax
// in outport.yml templates.
//
// The following variables are generated for each service:
//   - ${service.port}      — the allocated port number (e.g., "24920").
//   - ${service.env_var}   — the env_var name from config (e.g., "PORT").
//   - ${service.hostname}  — the .test hostname if one is configured, otherwise
//     the raw hostname from config or "localhost" as a fallback.
//   - ${service.url}       — the full URL for the service. If the service has an
//     active tunnel, this resolves to the tunnel URL (e.g., a Cloudflare URL).
//     Otherwise, it uses the .test hostname with the appropriate scheme.
//   - ${service.url:direct} — always resolves to http://localhost:{port}, bypassing
//     any tunnel. Useful when one local service needs to talk to another directly.
//
// Two standalone variables are also included:
//   - ${project_name} — the project name from outport.yml.
//   - ${instance}     — empty string for the main instance, or the instance code
//     (e.g., "bxcf") for worktree instances.
//
// The httpsEnabled flag controls the scheme for .test hostname URLs: when true,
// services with .test hostnames get https:// URLs (because the daemon's TLS proxy
// is active). Non-.test hostnames always use http://.
func BuildTemplateVars(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]string {
	vars := make(map[string]string)
	vars["project_name"] = cfg.Name
	if instanceName == "main" {
		vars["instance"] = ""
	} else {
		vars["instance"] = instanceName
	}
	for name, svc := range cfg.Services {
		portStr := fmt.Sprintf("%d", ports[name])
		vars[name+".port"] = portStr
		vars[name+".env_var"] = svc.EnvVar

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h

			if tunnelURL, hasTunnel := tunnelURLs[name]; hasTunnel {
				vars[name+".url"] = tunnelURL
			} else {
				vars[name+".url"] = fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(h, httpsEnabled), h)
			}
			vars[name+".url:direct"] = fmt.Sprintf("http://localhost:%s", portStr)
		} else {
			hostname := svc.Hostname
			if hostname == "" {
				hostname = "localhost"
			}
			vars[name+".hostname"] = hostname
		}

		// Alias template variables
		if svcAliases, ok := aliases[name]; ok {
			for key, aliasHostname := range svcAliases {
				vars[name+".alias."+key] = aliasHostname

				tunnelKey := name + "/alias/" + key
				if tunnelURL, hasTunnel := tunnelURLs[tunnelKey]; hasTunnel {
					vars[name+".alias_url."+key] = tunnelURL
				} else {
					vars[name+".alias_url."+key] = fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(aliasHostname, httpsEnabled), aliasHostname)
				}
			}
		}
	}
	return vars
}

// ResolveComputed resolves all computed value templates defined in the project's
// outport.yml config into their final string values. Computed values are env vars
// whose values are derived from other service attributes using ${service.field}
// template syntax (e.g., "http://localhost:${rails.port}/api/v1").
//
// It first builds the full template variable map via BuildTemplateVars, then
// delegates to config.ResolveComputed for the actual template substitution.
//
// The returned map is keyed as name -> file -> resolved value, where "name" is
// the computed value name (e.g., "API_URL"), "file" is the env file path it
// should be written to (e.g., "frontend/.env"), and the value is the fully
// resolved string with all ${...} references expanded.
//
// Returns nil if the config has no computed values defined.
func ResolveComputed(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, aliases map[string]map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]map[string]string {
	if len(cfg.Computed) == 0 {
		return nil
	}
	templateVars := BuildTemplateVars(cfg, instanceName, ports, hostnames, aliases, httpsEnabled, tunnelURLs)
	return config.ResolveComputed(cfg.Computed, templateVars)
}

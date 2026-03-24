// Package allocation builds registry allocations from config and ports.
// It handles hostname computation, protocol/env-var extraction,
// template variable building, and computed value resolution.
package allocation

import (
	"fmt"
	"strings"

	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/urlutil"
)

// Build constructs a registry Allocation from config, instance, directory, and ports.
func Build(cfg *config.Config, instanceName, dir string, ports map[string]int) registry.Allocation {
	return registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
		Hostnames:  ComputeHostnames(cfg, instanceName),
		Protocols:  computeProtocols(cfg),
		EnvVars:    computeEnvVars(cfg),
	}
}

// ComputeHostnames builds hostname map for an allocation.
// For "main" instance, hostnames are stem + ".test".
// For other instances, the project name in the stem is suffixed with "-instance".
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

// computeProtocols builds protocol map from config.
func computeProtocols(cfg *config.Config) map[string]string {
	protocols := make(map[string]string)
	for name, svc := range cfg.Services {
		if svc.Protocol != "" {
			protocols[name] = svc.Protocol
		}
	}
	return protocols
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

// BuildTemplateVars builds the template variable map from services and allocated ports.
// Keys are "service.field" (e.g., "rails.port", "rails.hostname", "rails.url").
// When httpsEnabled is true, .url uses https:// for .test hostnames.
// When tunnelURLs is non-nil, ${service.url} resolves to the tunnel URL for tunneled services.
// ${service.url:direct} always resolves to localhost (unaffected by tunnels).
func BuildTemplateVars(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]string {
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
		if svc.Protocol != "" {
			vars[name+".protocol"] = svc.Protocol
		}

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h
			protocol := svc.Protocol
			if protocol == "" {
				protocol = "http"
			}

			if tunnelURL, hasTunnel := tunnelURLs[name]; hasTunnel {
				vars[name+".url"] = tunnelURL
			} else {
				vars[name+".url"] = fmt.Sprintf("%s://%s", urlutil.EffectiveScheme(protocol, h, httpsEnabled), h)
			}
			vars[name+".url:direct"] = fmt.Sprintf("http://localhost:%s", portStr)
		} else {
			hostname := svc.Hostname
			if hostname == "" {
				hostname = "localhost"
			}
			vars[name+".hostname"] = hostname
		}
	}
	return vars
}

// ResolveComputed resolves computed value templates using allocated ports.
// Returns name → file → resolved value.
func ResolveComputed(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]map[string]string {
	if len(cfg.Computed) == 0 {
		return nil
	}
	templateVars := BuildTemplateVars(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
	return config.ResolveComputed(cfg.Computed, templateVars)
}

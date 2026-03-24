package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/steveclarke/outport/internal/urlutil"
)

// JSON types shared across multiple commands (up, ports, status, share).

type svcJSON struct {
	Port          int      `json:"port"`
	PreferredPort int      `json:"preferred_port,omitempty"`
	EnvVar        string   `json:"env_var"`
	Protocol      string   `json:"protocol,omitempty"`
	Hostname      string   `json:"hostname,omitempty"`
	URL           string   `json:"url,omitempty"`
	EnvFiles      []string `json:"env_files"`
	Up            *bool    `json:"up,omitempty"`
}

type computedJSON struct {
	Value    string            `json:"value,omitempty"`     // when all files share a value
	EnvFiles []string          `json:"env_files,omitempty"` // when all files share a value
	Values   map[string]string `json:"values,omitempty"`    // file → value when per-file
}

type upJSON struct {
	Project       string                  `json:"project"`
	Instance      string                  `json:"instance"`
	Services      map[string]svcJSON      `json:"services"`
	Computed      map[string]computedJSON `json:"computed,omitempty"`
	EnvFiles      []string                `json:"env_files"`
	ExternalFiles []externalFileJSON      `json:"external_files,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

// resolvedHostname returns the effective hostname for a service,
// preferring the allocated hostname over the config default.
func resolvedHostname(svc config.Service, hostnames map[string]string, name string) string {
	if h, ok := hostnames[name]; ok {
		return h
	}
	return svc.Hostname
}

// buildServiceMap builds the JSON service map used by up, ports, and status.
func buildServiceMap(cfg *config.Config, ports map[string]int, hostnames map[string]string, httpsEnabled bool) map[string]svcJSON {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		hostname := resolvedHostname(svc, hostnames, name)
		services[name] = svcJSON{
			Port:          ports[name],
			PreferredPort: svc.PreferredPort,
			EnvVar:        svc.EnvVar,
			Protocol:      svc.Protocol,
			Hostname:      hostname,
			URL:           urlutil.ServiceURL(svc.Protocol, hostname, ports[name], httpsEnabled),
			EnvFiles:      svc.EnvFiles,
		}
	}
	return services
}

// uniformValue returns the common value if all entries share the same value,
// or ("", false) if values differ across files.
func uniformValue(fileValues map[string]string) (string, bool) {
	var first string
	for _, v := range fileValues {
		if first == "" {
			first = v
		} else if v != first {
			return "", false
		}
	}
	return first, true
}

// buildComputedMap builds the JSON computed value map used by up, ports, status, and share.
func buildComputedMap(computed map[string]config.ComputedValue, resolved map[string]map[string]string) map[string]computedJSON {
	if len(resolved) == 0 {
		return nil
	}
	m := make(map[string]computedJSON)
	for name, fileValues := range resolved {
		dv := computed[name]
		if val, ok := uniformValue(fileValues); ok {
			m[name] = computedJSON{
				Value:    val,
				EnvFiles: dv.EnvFiles,
			}
		} else {
			m[name] = computedJSON{
				Values: fileValues,
			}
		}
	}
	return m
}

// printHeader renders the project name and instance badge.
func printHeader(w io.Writer, projectName, instanceName string) {
	header := ui.ProjectStyle.Render(projectName) + " " + ui.InstanceStyle.Render("["+instanceName+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// printComputedValues renders computed values with their resolved values.
func printComputedValues(w io.Writer, resolved map[string]map[string]string) {
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("    computed:"))
	names := slices.Sorted(maps.Keys(resolved))
	for _, name := range names {
		fileValues := resolved[name]
		if commonValue, allSame := uniformValue(fileValues); allSame {
			line := fmt.Sprintf("    %s  %s %s",
				ui.EnvVarStyle.Render(fmt.Sprintf("%-36s", name)),
				ui.Arrow,
				ui.DimStyle.Render(truncate(commonValue, 50)),
			)
			lipgloss.Fprintln(w, line)
		} else {
			lipgloss.Fprintln(w, fmt.Sprintf("    %s",
				ui.EnvVarStyle.Render(name)))
			files := slices.Sorted(maps.Keys(fileValues))
			for _, file := range files {
				line := fmt.Sprintf("      %s  %s %s",
					ui.DimStyle.Render(fmt.Sprintf("%-34s", file)),
					ui.Arrow,
					ui.DimStyle.Render(truncate(fileValues[file], 50)),
				)
				lipgloss.Fprintln(w, line)
			}
		}
	}
}

// printFlatServices renders the full service list with env var columns (used by up/ports).
func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, hostnames map[string]string, portStatus map[int]bool, httpsEnabled bool) {
	for _, svcName := range serviceNames {
		printServiceLineDetailed(w, cfg, svcName, ports[svcName], hostnames, portStatus, httpsEnabled)
	}
}

// serviceURLSuffix builds the URL or hostname suffix for a service line.
func serviceURLSuffix(cfg *config.Config, svcName string, hostnames map[string]string, port int, httpsEnabled bool) string {
	svc, ok := cfg.Services[svcName]
	if !ok {
		return ""
	}
	hostname := resolvedHostname(svc, hostnames, svcName)
	if u := urlutil.ServiceURL(svc.Protocol, hostname, port, httpsEnabled); u != "" {
		return "  " + ui.UrlStyle.Render(u)
	}
	if hostname != "" {
		return "  " + ui.HostnameStyle.Render(hostname)
	}
	return ""
}

// portStatusSuffix builds the up/down indicator for a service line.
func portStatusSuffix(portStatus map[int]bool, port int) string {
	if portStatus == nil {
		return ""
	}
	if portStatus[port] {
		return "  " + ui.StatusUp
	}
	return "  " + ui.StatusDown
}

// printServiceLineDetailed renders a service with env var column and 4-space indent (up/ports).
func printServiceLineDetailed(w io.Writer, cfg *config.Config, svcName string, port int, hostnames map[string]string, portStatus map[int]bool, httpsEnabled bool) {
	envVar := ""
	if svc, ok := cfg.Services[svcName]; ok {
		envVar = svc.EnvVar
	}
	line := fmt.Sprintf("    %s  %s  %s %-5s%s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", envVar)),
		ui.Arrow,
		ui.PortStyle.Render(fmt.Sprintf("%d", port)),
		portStatusSuffix(portStatus, port),
		serviceURLSuffix(cfg, svcName, hostnames, port, httpsEnabled),
	)
	lipgloss.Fprintln(w, line)
}

// printServiceLineCompact renders a service with 2-space indent, no env var (status).
func printServiceLineCompact(w io.Writer, cfg *config.Config, svcName string, port int, hostnames map[string]string, portStatus map[int]bool, httpsEnabled bool) {
	line := fmt.Sprintf("  %s  %s %-5s%s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.Arrow,
		ui.PortStyle.Render(fmt.Sprintf("%d", port)),
		portStatusSuffix(portStatus, port),
		serviceURLSuffix(cfg, svcName, hostnames, port, httpsEnabled),
	)
	lipgloss.Fprintln(w, line)
}

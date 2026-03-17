package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var forceFlag bool
var useHTTPS bool

var upCmd = &cobra.Command{
	Use:     "up",
	Short:   "Bring this project into outport",
	Long:    "Registers this project, allocates deterministic ports, saves to the central registry, and writes them to .env files.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runUp,
}

func init() {
	upCmd.Flags().BoolVar(&forceFlag, "force", false, "ignore existing allocations and re-allocate all ports")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	dir, cfg, reg := ctx.Dir, ctx.Cfg, ctx.Reg

	if ctx.IsNew && ctx.Instance != "main" {
		fmt.Printf("  Registered as %s-%s. Use 'outport rename %s <name>' to rename.\n\n",
			cfg.Name, ctx.Instance, ctx.Instance)
	}

	existing, hasExisting := reg.Get(cfg.Name, ctx.Instance)
	if forceFlag {
		hasExisting = false
	}

	usedPorts := reg.UsedPorts()
	if hasExisting {
		for _, port := range existing.Ports {
			delete(usedPorts, port)
		}
	} else {
		// When forcing, remove our old ports from usedPorts so preferred ports can be reclaimed
		if old, ok := reg.Get(cfg.Name, ctx.Instance); ok {
			for _, port := range old.Ports {
				delete(usedPorts, port)
			}
		}
	}

	ports := make(map[string]int)
	serviceNames := sortedMapKeys(cfg.Services)

	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		var port int

		if hasExisting {
			if existingPort, ok := existing.Ports[svcName]; ok {
				port = existingPort
				usedPorts[existingPort] = true
			}
		}

		if port == 0 {
			var err error
			port, err = allocator.Allocate(cfg.Name, ctx.Instance, svcName, svc.PreferredPort, usedPorts, portcheck.IsBound)
			if err != nil {
				return fmt.Errorf("allocating port for %s: %w", svcName, err)
			}
			usedPorts[port] = true
		}
		ports[svcName] = port
	}

	// Build allocation
	alloc := buildAllocation(cfg, ctx.Instance, dir, ports)

	// Check hostname uniqueness across registry
	for svcName, hostname := range alloc.Hostnames {
		projectKey := cfg.Name + "/" + ctx.Instance
		for regKey, regAlloc := range reg.Projects {
			if regKey == projectKey {
				continue // skip self
			}
			for _, existingHostname := range regAlloc.Hostnames {
				if existingHostname == hostname {
					return fmt.Errorf("hostname %q (service %q) conflicts with %s", hostname, svcName, regKey)
				}
			}
		}
	}

	reg.Set(cfg.Name, ctx.Instance, alloc)
	if err := reg.Save(); err != nil {
		return err
	}

	useHTTPS = certmanager.IsCAInstalled()

	if err := mergeEnvFiles(dir, cfg, ports, alloc.Hostnames); err != nil {
		return err
	}

	resolvedDerived := resolveDerivedFromAlloc(cfg, ports, alloc.Hostnames)
	envFiles := mergedEnvFileList(cfg, resolvedDerived)

	if jsonFlag {
		return printUpJSON(cmd, cfg, ctx.Instance, ports, alloc.Hostnames, resolvedDerived, envFiles)
	}

	if err := printUpStyled(cmd, cfg, ctx.Instance, serviceNames, ports, alloc.Hostnames, resolvedDerived, envFiles); err != nil {
		return err
	}

	if !platform.IsAgentLoaded() {
		w := cmd.OutOrStdout()
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.DimStyle.Render("Hint: The outport daemon is not running. Run 'outport system start' to enable .test domains."))
	}

	return nil
}

// mergedEnvFileList returns the sorted list of env files that would be written
// by mergeEnvFiles, for display purposes.
func mergedEnvFileList(cfg *config.Config, resolvedDerived map[string]map[string]string) []string {
	files := make(map[string]bool)
	for _, svc := range cfg.Services {
		for _, envFile := range svc.EnvFiles {
			files[envFile] = true
		}
	}
	for _, fileValues := range resolvedDerived {
		for file := range fileValues {
			files[file] = true
		}
	}
	return sortedMapKeys(files)
}

// buildAllocation constructs a registry Allocation from config, instance, directory, and ports.
func buildAllocation(cfg *config.Config, instanceName, dir string, ports map[string]int) registry.Allocation {
	return registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
		Hostnames:  computeHostnames(cfg, instanceName),
		Protocols:  computeProtocols(cfg),
	}
}

// resolvedHostname returns the effective hostname for a service,
// preferring the allocated hostname over the config default.
func resolvedHostname(svc config.Service, hostnames map[string]string, name string) string {
	if h, ok := hostnames[name]; ok {
		return h
	}
	return svc.Hostname
}

// computeHostnames builds hostname map for an allocation.
// For "main" instance, hostnames are stem + ".test".
// For other instances, the project name in the stem is suffixed with "-instance".
func computeHostnames(cfg *config.Config, instanceName string) map[string]string {
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

// effectiveScheme returns the scheme to use for a given protocol and hostname.
// When useHTTPS is true and the hostname is a .test domain with an HTTP protocol,
// the scheme is upgraded to "https".
func effectiveScheme(protocol, hostname string) string {
	if useHTTPS && strings.HasSuffix(hostname, ".test") && (protocol == "http" || protocol == "https") {
		return "https"
	}
	return protocol
}

// buildTemplateVars builds the template variable map from services and allocated ports.
// Keys are "service.field" (e.g., "rails.port", "rails.hostname", "rails.url").
// When useHTTPS is true, .url uses https:// for .test hostnames.
func buildTemplateVars(cfg *config.Config, ports map[string]int, hostnames map[string]string) map[string]string {
	vars := make(map[string]string)
	for name, svc := range cfg.Services {
		portStr := fmt.Sprintf("%d", ports[name])
		vars[name+".port"] = portStr

		if h, ok := hostnames[name]; ok {
			vars[name+".hostname"] = h
			protocol := svc.Protocol
			if protocol == "" {
				protocol = "http"
			}
			vars[name+".url"] = fmt.Sprintf("%s://%s", effectiveScheme(protocol, h), h)
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

// resolveDerivedFromAlloc resolves derived value templates using allocated ports.
// Returns name → file → resolved value.
func resolveDerivedFromAlloc(cfg *config.Config, ports map[string]int, hostnames map[string]string) map[string]map[string]string {
	if len(cfg.Derived) == 0 {
		return nil
	}
	templateVars := buildTemplateVars(cfg, ports, hostnames)
	return config.ResolveDerived(cfg.Derived, templateVars)
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// JSON types

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

func boolPtr(b bool) *bool { return &b }

type derivedJSON struct {
	Value    string            `json:"value,omitempty"`     // when all files share a value
	EnvFiles []string          `json:"env_files,omitempty"` // when all files share a value
	Values   map[string]string `json:"values,omitempty"`    // file → value when per-file
}

type upJSON struct {
	Project  string                 `json:"project"`
	Instance string                 `json:"instance"`
	Services map[string]svcJSON     `json:"services"`
	Derived  map[string]derivedJSON `json:"derived,omitempty"`
	EnvFiles []string               `json:"env_files"`
}

func serviceURL(protocol, hostname string, port int) string {
	if protocol == "http" || protocol == "https" {
		host := hostname
		if host == "" {
			host = "localhost"
		}
		if strings.HasSuffix(host, ".test") {
			return fmt.Sprintf("%s://%s", effectiveScheme(protocol, host), host)
		}
		return fmt.Sprintf("%s://%s:%d", protocol, host, port)
	}
	return ""
}

func buildServiceMap(cfg *config.Config, ports map[string]int, hostnames map[string]string) map[string]svcJSON {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		hostname := resolvedHostname(svc, hostnames, name)
		services[name] = svcJSON{
			Port:          ports[name],
			PreferredPort: svc.PreferredPort,
			EnvVar:        svc.EnvVar,
			Protocol:      svc.Protocol,
			Hostname:      hostname,
			URL:           serviceURL(svc.Protocol, hostname, ports[name]),
			EnvFiles:      svc.EnvFiles,
		}
	}
	return services
}

func buildDerivedMap(derived map[string]config.DerivedValue, resolved map[string]map[string]string) map[string]derivedJSON {
	if len(resolved) == 0 {
		return nil
	}
	m := make(map[string]derivedJSON)
	for name, fileValues := range resolved {
		dv := derived[name]
		// If all files have the same resolved value, use the simple format
		allSame := true
		var commonValue string
		for _, v := range fileValues {
			if commonValue == "" {
				commonValue = v
			} else if v != commonValue {
				allSame = false
				break
			}
		}
		if allSame {
			m[name] = derivedJSON{
				Value:    commonValue,
				EnvFiles: dv.EnvFiles,
			}
		} else {
			m[name] = derivedJSON{
				Values: fileValues,
			}
		}
	}
	return m
}

func printUpJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, resolvedDerived map[string]map[string]string, envFiles []string) error {
	out := upJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Services: buildServiceMap(cfg, ports, hostnames),
		Derived:  buildDerivedMap(cfg.Derived, resolvedDerived),
		EnvFiles: envFiles,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printHeader(w io.Writer, projectName, instanceName string) {
	header := ui.ProjectStyle.Render(projectName) + " " + ui.InstanceStyle.Render("["+instanceName+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)
}

func printUpStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, serviceNames []string, ports map[string]int, hostnames map[string]string, resolvedDerived map[string]map[string]string, envFiles []string) error {
	w := cmd.OutOrStdout()

	printHeader(w, cfg.Name, instanceName)

	printFlatServices(w, cfg, serviceNames, ports, hostnames, nil)

	if len(resolvedDerived) > 0 {
		printDerivedValues(w, resolvedDerived)
	}

	lipgloss.Fprintln(w)
	if len(envFiles) == 1 {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("Ports written to "+envFiles[0]))
	} else {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("Ports written to:"))
		for _, f := range envFiles {
			lipgloss.Fprintln(w, ui.SuccessStyle.Render("  "+f))
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func printDerivedValues(w io.Writer, resolved map[string]map[string]string) {
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("    derived:"))
	names := sortedMapKeys(resolved)
	for _, name := range names {
		fileValues := resolved[name]
		// Check if all files have the same value
		allSame := true
		var commonValue string
		for _, v := range fileValues {
			if commonValue == "" {
				commonValue = v
			} else if v != commonValue {
				allSame = false
				break
			}
		}
		if allSame {
			line := fmt.Sprintf("    %s  %s %s",
				ui.EnvVarStyle.Render(fmt.Sprintf("%-36s", name)),
				ui.Arrow,
				ui.DimStyle.Render(truncate(commonValue, 50)),
			)
			lipgloss.Fprintln(w, line)
		} else {
			lipgloss.Fprintln(w, fmt.Sprintf("    %s",
				ui.EnvVarStyle.Render(name)))
			files := sortedMapKeys(fileValues)
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

func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, hostnames map[string]string, portStatus map[int]bool) {
	for _, svcName := range serviceNames {
		printServiceLine(w, cfg, svcName, ports[svcName], hostnames, portStatus)
	}
}

func printServiceLine(w io.Writer, cfg *config.Config, svcName string, port int, hostnames map[string]string, portStatus map[int]bool) {
	svc := cfg.Services[svcName]

	status := ""
	if portStatus != nil {
		if portStatus[port] {
			status = "  " + ui.StatusUp
		} else {
			status = "  " + ui.StatusDown
		}
	}

	hostname := resolvedHostname(svc, hostnames, svcName)

	extra := ""
	if u := serviceURL(svc.Protocol, hostname, port); u != "" {
		extra = "  " + ui.UrlStyle.Render(u)
	} else if hostname != "" {
		extra = "  " + ui.HostnameStyle.Render(hostname)
	}

	line := fmt.Sprintf("    %s  %s  %s %-5s%s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", svc.EnvVar)),
		ui.Arrow,
		ui.PortStyle.Render(fmt.Sprintf("%d", port)),
		status,
		extra,
	)
	lipgloss.Fprintln(w, line)
}

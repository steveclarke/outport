package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/envpath"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/urlutil"
	"github.com/spf13/cobra"
)

var forceFlag bool

// isPortBusy checks if a port is in use on the system. Tests can override this
// to avoid flaky failures when common ports (e.g., 5432) are bound locally.
var isPortBusy = portcheck.IsBound

var upCmd = &cobra.Command{
	Use:     "up",
	Short:   "Bring this project into outport",
	Long:    "Registers this project, allocates deterministic ports, saves to the central registry, and writes them to .env files.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runUp,
}

func init() {
	upCmd.Flags().BoolVar(&forceFlag, "force", false, "re-allocate all ports and reset external file approvals")
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
			port, err = allocator.Allocate(cfg.Name, ctx.Instance, svcName, svc.PreferredPort, usedPorts, isPortBusy)
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

	httpsEnabled := certmanager.IsCAInstalled()

	// Get approved paths from existing allocation; clear if --force.
	var approvedPaths []string
	if !forceFlag && hasExisting {
		approvedPaths = existing.ApprovedExternalFiles
	}

	result, err := writeEnvFiles(dir, cfg, ctx.Instance, ports, alloc.Hostnames, httpsEnabled, nil,
		yesFlag, approvedPaths, os.Stdin, os.Stderr)
	if err != nil {
		return err
	}

	// Update allocation with newly approved paths and save
	if len(result.NewlyApproved) > 0 {
		alloc.ApprovedExternalFiles = mergeApprovedPaths(approvedPaths, result.NewlyApproved)
		reg.Set(cfg.Name, ctx.Instance, alloc)
	}

	if err := reg.Save(); err != nil {
		return err
	}

	envFiles := mergedEnvFileList(cfg, result.ResolvedComputed)

	if jsonFlag {
		return printUpJSON(cmd, cfg, ctx.Instance, ports, alloc.Hostnames, result.ResolvedComputed, envFiles, httpsEnabled, result.ExternalFiles)
	}

	if err := printUpStyled(cmd, cfg, ctx.Instance, serviceNames, ports, alloc.Hostnames, result.ResolvedComputed, envFiles, httpsEnabled); err != nil {
		return err
	}

	printExternalFilesWarning(cmd.OutOrStdout(), result.ExternalFiles)

	w := cmd.OutOrStdout()
	if !platform.IsAgentLoaded() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.DimStyle.Render("Hint: The outport daemon is not running. Run 'outport system start' to enable .test domains."))
	} else {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.DimStyle.Render("Dashboard: https://outport.test"))
	}

	return nil
}

// mergedEnvFileList returns the sorted list of env files that would be written
// by mergeEnvFiles, for display purposes.
func mergedEnvFileList(cfg *config.Config, resolvedComputed map[string]map[string]string) []string {
	files := make(map[string]bool)
	for _, svc := range cfg.Services {
		for _, envFile := range svc.EnvFiles {
			files[envFile] = true
		}
	}
	for _, fileValues := range resolvedComputed {
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
		EnvVars:    computeEnvVars(cfg),
	}
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


// buildTemplateVars builds the template variable map from services and allocated ports.
// Keys are "service.field" (e.g., "rails.port", "rails.hostname", "rails.url").
// When httpsEnabled is true, .url uses https:// for .test hostnames.
// When tunnelURLs is non-nil, ${service.url} resolves to the tunnel URL for tunneled services.
// ${service.url:direct} always resolves to localhost (unaffected by tunnels).
func buildTemplateVars(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]string {
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

// resolveComputedFromAlloc resolves computed value templates using allocated ports.
// Returns name → file → resolved value.
func resolveComputedFromAlloc(cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, httpsEnabled bool, tunnelURLs map[string]string) map[string]map[string]string {
	if len(cfg.Computed) == 0 {
		return nil
	}
	templateVars := buildTemplateVars(cfg, instanceName, ports, hostnames, httpsEnabled, tunnelURLs)
	return config.ResolveComputed(cfg.Computed, templateVars)
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

func printUpJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, ports map[string]int, hostnames map[string]string, resolvedComputed map[string]map[string]string, envFiles []string, httpsEnabled bool, externalFiles []envpath.EnvFilePath) error {
	out := upJSON{
		Project:       cfg.Name,
		Instance:      instanceName,
		Services:      buildServiceMap(cfg, ports, hostnames, httpsEnabled),
		Computed:      buildComputedMap(cfg.Computed, resolvedComputed),
		EnvFiles:      envFiles,
		ExternalFiles: toExternalFileJSON(externalFiles),
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

func printUpStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, serviceNames []string, ports map[string]int, hostnames map[string]string, resolvedComputed map[string]map[string]string, envFiles []string, httpsEnabled bool) error {
	w := cmd.OutOrStdout()

	printHeader(w, cfg.Name, instanceName)

	printFlatServices(w, cfg, serviceNames, ports, hostnames, nil, httpsEnabled)

	if len(resolvedComputed) > 0 {
		printComputedValues(w, resolvedComputed)
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

func printComputedValues(w io.Writer, resolved map[string]map[string]string) {
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("    computed:"))
	names := sortedMapKeys(resolved)
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

func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, hostnames map[string]string, portStatus map[int]bool, httpsEnabled bool) {
	for _, svcName := range serviceNames {
		printServiceLine(w, cfg, svcName, ports[svcName], hostnames, portStatus, httpsEnabled, true)
	}
}

// printServiceLine renders a single service line. When showEnvVar is true,
// the env var column is included with a 4-space indent (used by up/ports).
// When false, the line uses a 2-space indent without the env var (used by status).
func printServiceLine(w io.Writer, cfg *config.Config, svcName string, port int, hostnames map[string]string, portStatus map[int]bool, httpsEnabled bool, showEnvVar bool) {
	svc, ok := cfg.Services[svcName]

	status := ""
	if portStatus != nil {
		if portStatus[port] {
			status = "  " + ui.StatusUp
		} else {
			status = "  " + ui.StatusDown
		}
	}

	extra := ""
	if ok {
		hostname := resolvedHostname(svc, hostnames, svcName)
		if u := urlutil.ServiceURL(svc.Protocol, hostname, port, httpsEnabled); u != "" {
			extra = "  " + ui.UrlStyle.Render(u)
		} else if hostname != "" {
			extra = "  " + ui.HostnameStyle.Render(hostname)
		}
	}

	var line string
	if showEnvVar {
		envVar := ""
		if ok {
			envVar = svc.EnvVar
		}
		line = fmt.Sprintf("    %s  %s  %s %-5s%s%s",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
			ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", envVar)),
			ui.Arrow,
			ui.PortStyle.Render(fmt.Sprintf("%d", port)),
			status,
			extra,
		)
	} else {
		line = fmt.Sprintf("  %s  %s %-5s%s%s",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
			ui.Arrow,
			ui.PortStyle.Render(fmt.Sprintf("%d", port)),
			status,
			extra,
		)
	}
	lipgloss.Fprintln(w, line)
}

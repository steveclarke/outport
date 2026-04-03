package cmd

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/portinfo"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/steveclarke/outport/internal/urlutil"
)

var portsAllFlag bool

// portScanner is the scanner used by the ports command. Tests can replace it.
var portScanner portinfo.Scanner = portinfo.SystemScanner{}

var portsCmd = &cobra.Command{
	Use:     "ports",
	Short:   "Show port allocations and running processes",
	Long:    "Shows port allocations with live process information. Inside a project directory, shows the current project. Outside, shows all registered projects. Use --all for a full machine scan.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runPorts,
}

func init() {
	portsCmd.Flags().BoolVar(&portsAllFlag, "all", false, "scan all listening ports on the machine")
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	if portsAllFlag {
		return runPortsAll(cmd)
	}

	ctx, err := loadProjectContext()
	if err != nil {
		// Outside a project directory — show all-Outport view
		return runPortsAllOutport(cmd)
	}

	return runPortsProject(cmd, ctx)
}

// --- Project view ---

func runPortsProject(cmd *cobra.Command, ctx *projectContext) error {
	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		fmt.Fprintln(cmd.OutOrStdout(), "No ports allocated. Run 'outport up' first.")
		return nil
	}

	// Scan only this project's ports
	var allPorts []int
	for _, port := range alloc.Ports {
		allPorts = append(allPorts, port)
	}

	procs, err := portinfo.ScanPorts(allPorts, portScanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}
	byPort := indexByPort(procs)

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsProjectJSON(cmd, ctx.Cfg, ctx.Instance, alloc, byPort, httpsEnabled)
	}
	return printPortsProjectStyled(cmd, ctx.Cfg, ctx.Instance, alloc, byPort, httpsEnabled)
}

func printPortsProjectStyled(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	printHeader(w, cfg.Name, instanceName)

	serviceNames := slices.Sorted(maps.Keys(alloc.Ports))
	for _, svcName := range serviceNames {
		port := alloc.Ports[svcName]
		portStatus := map[int]bool{port: byPort[port].PID > 0}
		printServiceLineDetailed(w, cfg, svcName, port, alloc.Hostnames, portStatus, httpsEnabled)
		if proc, ok := byPort[port]; ok {
			printProcessLine(w, proc)
		}
		if svcAliases, ok := alloc.Aliases[svcName]; ok {
			printAliasLines(w, svcAliases, port, httpsEnabled)
		}
	}

	return nil
}

func printPortsProjectJSON(cmd *cobra.Command, cfg *config.Config, instanceName string, alloc registry.Allocation, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	var ports []portEntryJSON
	serviceNames := slices.Sorted(maps.Keys(alloc.Ports))
	key := registry.Key(cfg.Name, instanceName)

	for _, svcName := range serviceNames {
		port := alloc.Ports[svcName]
		hostname := ""
		if h, ok := alloc.Hostnames[svcName]; ok {
			hostname = h
		} else if svc, ok := cfg.Services[svcName]; ok {
			hostname = svc.Hostname
		}
		entry := portEntryJSON{
			Port:        port,
			Service:     svcName,
			RegistryKey: key,
			Hostname:    hostname,
			URL:         urlutil.ServiceURL(hostname, port, httpsEnabled),
			Up:          byPort[port].PID > 0,
		}
		if proc, ok := byPort[port]; ok {
			entry.Process = toPortProcessJSON(proc)
		}
		ports = append(ports, entry)
	}

	out := portsProjectJSON{
		Project:  cfg.Name,
		Instance: instanceName,
		Ports:    ports,
	}
	n := len(ports)
	summary := fmt.Sprintf("%d %s", n, pluralize(n, "port", "ports"))
	return writeJSON(cmd, out, summary)
}

// --- All-Outport view ---

func runPortsAllOutport(cmd *cobra.Command) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	projects := reg.All()
	if len(projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Run 'outport up' in a project directory.")
		return nil
	}

	// Collect all managed ports for scanning
	var allPorts []int
	for _, alloc := range projects {
		for _, port := range alloc.Ports {
			allPorts = append(allPorts, port)
		}
	}

	procs, err := portinfo.ScanPorts(allPorts, portScanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}
	byPort := indexByPort(procs)

	httpsEnabled := certmanager.IsCAInstalled()

	if jsonFlag {
		return printPortsAllOutportJSON(cmd, reg, projects, byPort, httpsEnabled)
	}
	return printPortsAllOutportStyled(cmd, reg, projects, byPort, httpsEnabled)
}

func printPortsAllOutportStyled(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	w := cmd.OutOrStdout()
	currentKey := currentProjectKey(reg)
	keys := slices.Sorted(maps.Keys(projects))

	for i, key := range keys {
		alloc := projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)

		stale, staleReason := isStale(alloc.ProjectDir)

		marker := ""
		if key == currentKey {
			marker = currentMarker.Render(" ●")
		}
		if stale {
			marker += " " + ui.DimStyle.Render(staleReason)
		}
		displayName := formatProjectKey(key)
		header := ui.ProjectStyle.Render(displayName) + " " + ui.DimStyle.Render(alloc.ProjectDir) + marker
		lipgloss.Fprintln(w, header)

		renderCfg := cfg
		if renderCfg == nil {
			renderCfg = &config.Config{Services: make(map[string]config.Service)}
		}

		svcNames := slices.Sorted(maps.Keys(alloc.Ports))
		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]
			portStatus := map[int]bool{port: byPort[port].PID > 0}
			printServiceLineCompact(w, renderCfg, svcName, port, alloc.Hostnames, portStatus, httpsEnabled)
			if proc, ok := byPort[port]; ok {
				printProcessLine(w, proc)
			}
			if svcAliases, ok := alloc.Aliases[svcName]; ok {
				printAliasLines(w, svcAliases, port, httpsEnabled)
			}
		}

		if stale {
			fmt.Fprintf(w, "  %s\n", ui.DimStyle.Render("(stale — run 'outport system prune' to remove)"))
		}

		if i < len(keys)-1 {
			lipgloss.Fprintln(w)
		}
	}

	return nil
}

func printPortsAllOutportJSON(cmd *cobra.Command, reg *registry.Registry, projects map[string]registry.Allocation, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	var entries []portsProjectJSON
	keys := slices.Sorted(maps.Keys(projects))

	for _, key := range keys {
		alloc := projects[key]
		project, instanceName := registry.ParseKey(key)
		cfg := loadProjectConfig(alloc.ProjectDir)

		var ports []portEntryJSON
		svcNames := slices.Sorted(maps.Keys(alloc.Ports))
		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]
			hostname := ""
			if h, ok := alloc.Hostnames[svcName]; ok {
				hostname = h
			} else if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					hostname = svc.Hostname
				}
			}
			entry := portEntryJSON{
				Port:        port,
				Service:     svcName,
				RegistryKey: key,
				Hostname:    hostname,
				URL:         urlutil.ServiceURL(hostname, port, httpsEnabled),
				Up:          byPort[port].PID > 0,
			}
			if proc, ok := byPort[port]; ok {
				entry.Process = toPortProcessJSON(proc)
			}
			ports = append(ports, entry)
		}

		entries = append(entries, portsProjectJSON{
			Project:  project,
			Instance: instanceName,
			Ports:    ports,
		})
	}

	n := len(entries)
	summary := fmt.Sprintf("%d %s", n, pluralize(n, "project", "projects"))
	return writeJSON(cmd, entries, summary)
}

// --- Full machine scan ---

type managedPort struct {
	key      string
	service  string
	hostname string
	port     int
}

func runPortsAll(cmd *cobra.Command) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	procs, err := portinfo.Scan(portScanner)
	if err != nil {
		return fmt.Errorf("scanning ports: %w", err)
	}
	byPort := indexByPort(procs)

	projects := reg.All()
	httpsEnabled := certmanager.IsCAInstalled()

	// Build a set of managed ports and their registry metadata
	var managed []managedPort
	managedPorts := make(map[int]bool)

	keys := slices.Sorted(maps.Keys(projects))
	for _, key := range keys {
		alloc := projects[key]
		cfg := loadProjectConfig(alloc.ProjectDir)
		svcNames := slices.Sorted(maps.Keys(alloc.Ports))
		for _, svcName := range svcNames {
			port := alloc.Ports[svcName]
			hostname := ""
			if h, ok := alloc.Hostnames[svcName]; ok {
				hostname = h
			} else if cfg != nil {
				if svc, ok := cfg.Services[svcName]; ok {
					hostname = svc.Hostname
				}
			}
			managed = append(managed, managedPort{
				key:      key,
				service:  svcName,
				hostname: hostname,
				port:     port,
			})
			managedPorts[port] = true
		}
	}

	// Collect non-managed processes
	var other []portinfo.ProcessInfo
	for _, proc := range procs {
		if !managedPorts[proc.Port] {
			other = append(other, proc)
		}
	}

	if jsonFlag {
		return printPortsAllJSON(cmd, managed, other, byPort, httpsEnabled)
	}
	return printPortsAllStyled(cmd, managed, other, byPort, httpsEnabled)
}

func printPortsAllStyled(cmd *cobra.Command, managed []managedPort, other []portinfo.ProcessInfo, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	w := cmd.OutOrStdout()

	if len(managed) > 0 {
		lipgloss.Fprintln(w, ui.ProjectStyle.Render("Outport managed"))
		lipgloss.Fprintln(w)

		for _, m := range managed {
			up := byPort[m.port].PID > 0
			status := "  " + ui.StatusDown
			if up {
				status = "  " + ui.StatusUp
			}
			urlSuffix := ""
			if u := urlutil.ServiceURL(m.hostname, m.port, httpsEnabled); u != "" {
				urlSuffix = "  " + ui.UrlStyle.Render(u)
			}
			line := fmt.Sprintf("  %s  %s  %s %-5s%s%s",
				ui.ServiceStyle.Render(fmt.Sprintf("%-16s", m.service)),
				ui.DimStyle.Render(fmt.Sprintf("%-20s", formatProjectKey(m.key))),
				ui.Arrow,
				ui.PortStyle.Render(fmt.Sprintf("%d", m.port)),
				status,
				urlSuffix,
			)
			lipgloss.Fprintln(w, line)
			if proc, ok := byPort[m.port]; ok {
				printProcessLine(w, proc)
			}
		}
	}

	if len(other) > 0 {
		if len(managed) > 0 {
			lipgloss.Fprintln(w)
		}
		lipgloss.Fprintln(w, ui.ProjectStyle.Render("Other"))
		lipgloss.Fprintln(w)

		for _, proc := range other {
			line := fmt.Sprintf("  %s  %s %-5s",
				ui.ServiceStyle.Render(fmt.Sprintf("%-16s", proc.Name)),
				ui.Arrow,
				ui.PortStyle.Render(fmt.Sprintf("%d", proc.Port)),
			)
			lipgloss.Fprintln(w, line)
			printProcessLine(w, proc)
		}
	}

	if len(managed) == 0 && len(other) == 0 {
		fmt.Fprintln(w, "No listening ports found.")
	}

	return nil
}

func printPortsAllJSON(cmd *cobra.Command, managed []managedPort, other []portinfo.ProcessInfo, byPort map[int]portinfo.ProcessInfo, httpsEnabled bool) error {
	var managedEntries []portEntryJSON
	for _, m := range managed {
		entry := portEntryJSON{
			Port:        m.port,
			Service:     m.service,
			RegistryKey: m.key,
			Hostname:    m.hostname,
			URL:         urlutil.ServiceURL(m.hostname, m.port, httpsEnabled),
			Up:          byPort[m.port].PID > 0,
		}
		if proc, ok := byPort[m.port]; ok {
			entry.Process = toPortProcessJSON(proc)
		}
		managedEntries = append(managedEntries, entry)
	}

	var otherEntries []portEntryJSON
	for _, proc := range other {
		entry := portEntryJSON{
			Port: proc.Port,
			Up:   true,
			Process: &portProcessJSON{
				PID:           proc.PID,
				PPID:          proc.PPID,
				Name:          proc.Name,
				Command:       proc.Command,
				RSSBytes:      proc.RSS,
				UptimeSeconds: proc.UptimeSeconds(),
				CWD:           proc.CWD,
				Project:       proc.Project,
				Framework:     proc.Framework,
				IsOrphan:      proc.IsOrphan,
				IsZombie:      proc.IsZombie,
			},
		}
		otherEntries = append(otherEntries, entry)
	}

	out := portsAllJSON{
		Managed: managedEntries,
		Other:   otherEntries,
	}

	total := len(managedEntries) + len(otherEntries)
	summary := fmt.Sprintf("%d %s (%d managed, %d other)",
		total, pluralize(total, "port", "ports"),
		len(managedEntries), len(otherEntries))
	return writeJSON(cmd, out, summary)
}

// --- Helper functions ---

// indexByPort builds a lookup map from port number to ProcessInfo.
func indexByPort(procs []portinfo.ProcessInfo) map[int]portinfo.ProcessInfo {
	m := make(map[int]portinfo.ProcessInfo, len(procs))
	for _, p := range procs {
		m[p.Port] = p
	}
	return m
}

// printProcessLine renders a compact process detail line underneath a service.
func printProcessLine(w io.Writer, proc portinfo.ProcessInfo) {
	uptime := formatUptime(time.Duration(proc.UptimeSeconds()) * time.Second)
	mem := formatMemory(proc.RSS)

	parts := fmt.Sprintf("PID %d", proc.PID)
	if proc.Name != "" {
		parts += " · " + proc.Name
	}
	if proc.Command != "" && proc.Command != proc.Name {
		parts += " (" + truncate(proc.Command, 40) + ")"
	}
	if mem != "" {
		parts += " · " + mem
	}
	if uptime != "" {
		parts += " · " + uptime
	}

	var warnings []string
	if proc.IsOrphan {
		warnings = append(warnings, "orphan")
	}
	if proc.IsZombie {
		warnings = append(warnings, "zombie")
	}

	line := "      " + ui.DimStyle.Render(parts)
	if len(warnings) > 0 {
		for _, w := range warnings {
			line += "  " + ui.WarnStyle.Render("⚠ "+w)
		}
	}
	lipgloss.Fprintln(w, line)
}

// formatMemory formats bytes into a human-readable string.
func formatMemory(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%d MB", bytes/mb)
	default:
		return fmt.Sprintf("%d KB", bytes/1024)
	}
}

// formatUptime formats a duration into a compact human-readable string.
func formatUptime(d time.Duration) string {
	if d <= 0 {
		return ""
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return "<1m"
	}
}

// toPortProcessJSON converts a ProcessInfo to a JSON-serializable struct.
func toPortProcessJSON(proc portinfo.ProcessInfo) *portProcessJSON {
	return &portProcessJSON{
		PID:           proc.PID,
		PPID:          proc.PPID,
		Name:          proc.Name,
		Command:       proc.Command,
		RSSBytes:      proc.RSS,
		UptimeSeconds: proc.UptimeSeconds(),
		CWD:           proc.CWD,
		Project:       proc.Project,
		Framework:     proc.Framework,
		IsOrphan:      proc.IsOrphan,
		IsZombie:      proc.IsZombie,
	}
}

// --- JSON types ---

type portProcessJSON struct {
	PID           int    `json:"pid"`
	PPID          int    `json:"ppid"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	RSSBytes      int64  `json:"rss_bytes"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	CWD           string `json:"cwd,omitempty"`
	Project       string `json:"project,omitempty"`
	Framework     string `json:"framework,omitempty"`
	IsOrphan      bool   `json:"is_orphan"`
	IsZombie      bool   `json:"is_zombie"`
}

type portEntryJSON struct {
	Port        int              `json:"port"`
	Service     string           `json:"service,omitempty"`
	RegistryKey string           `json:"registry_key,omitempty"`
	Hostname    string           `json:"hostname,omitempty"`
	URL         string           `json:"url,omitempty"`
	Up          bool             `json:"up"`
	Process     *portProcessJSON `json:"process,omitempty"`
}

type portsProjectJSON struct {
	Project  string          `json:"project"`
	Instance string          `json:"instance"`
	Ports    []portEntryJSON `json:"ports"`
}

type portsAllJSON struct {
	Managed []portEntryJSON `json:"managed"`
	Other   []portEntryJSON `json:"other"`
}

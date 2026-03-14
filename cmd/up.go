package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Allocate ports and write to .env files",
	Long:  "Reads .outport.yml, allocates deterministic ports, and writes them to .env files.",
	RunE:  runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(dir)
	if err != nil {
		return fmt.Errorf("detecting worktree: %w", err)
	}

	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	existing, hasExisting := reg.Get(cfg.Name, wt.Instance)

	usedPorts := reg.UsedPorts()
	if hasExisting {
		for _, port := range existing.Ports {
			delete(usedPorts, port)
		}
	}

	ports := make(map[string]int)
	envFileVars := make(map[string]map[string]string)

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
			port, err = allocator.Allocate(cfg.Name, wt.Instance, svcName, svc.PreferredPort, usedPorts)
			if err != nil {
				return fmt.Errorf("allocating port for %s: %w", svcName, err)
			}
			usedPorts[port] = true
		}
		ports[svcName] = port

		for _, envFile := range svc.EnvFiles {
			if envFileVars[envFile] == nil {
				envFileVars[envFile] = make(map[string]string)
			}
			envFileVars[envFile][svc.EnvVar] = fmt.Sprintf("%d", port)
		}
	}

	reg.Set(cfg.Name, wt.Instance, registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
	})
	if err := reg.Save(); err != nil {
		return err
	}

	envFiles := sortedMapKeys(envFileVars)
	for _, envFile := range envFiles {
		envPath := filepath.Join(dir, envFile)
		if err := dotenv.Merge(envPath, envFileVars[envFile]); err != nil {
			return fmt.Errorf("writing %s: %w", envFile, err)
		}
	}

	if jsonFlag {
		return printUpJSON(cmd, cfg, wt, ports, envFiles)
	}
	return printUpStyled(cmd, cfg, wt, serviceNames, ports, envFiles)
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
	URL           string   `json:"url,omitempty"`
	EnvFiles      []string `json:"env_files"`
	Group         string   `json:"group,omitempty"`
	Up            *bool    `json:"up,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

type upJSON struct {
	Project  string             `json:"project"`
	Instance string             `json:"instance"`
	Services map[string]svcJSON `json:"services"`
	EnvFiles []string           `json:"env_files"`
}

func serviceURL(protocol string, port int) string {
	if protocol == "http" || protocol == "https" {
		return fmt.Sprintf("%s://localhost:%d", protocol, port)
	}
	return ""
}

func buildServiceMap(cfg *config.Config, ports map[string]int) map[string]svcJSON {
	services := make(map[string]svcJSON)
	for name, svc := range cfg.Services {
		services[name] = svcJSON{
			Port:          ports[name],
			PreferredPort: svc.PreferredPort,
			EnvVar:        svc.EnvVar,
			Protocol:      svc.Protocol,
			URL:           serviceURL(svc.Protocol, ports[name]),
			EnvFiles:      svc.EnvFiles,
			Group:         svc.Group,
		}
	}
	return services
}

func printUpJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, ports map[string]int, envFiles []string) error {
	out := upJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: buildServiceMap(cfg, ports),
		EnvFiles: envFiles,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printHeader(w io.Writer, projectName string, wt *worktree.Info) {
	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}
	header := ui.ProjectStyle.Render(projectName) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)
}

func printUpStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, serviceNames []string, ports map[string]int, envFiles []string) error {
	w := cmd.OutOrStdout()

	printHeader(w, cfg.Name, wt)

	hasGroups := false
	for _, svcName := range serviceNames {
		if cfg.Services[svcName].Group != "" {
			hasGroups = true
			break
		}
	}

	if hasGroups {
		printGroupedServices(w, cfg, serviceNames, ports, false)
	} else {
		printFlatServices(w, cfg, serviceNames, ports, false)
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

func printGroupedServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, check bool) {
	var ungrouped []string
	groupServices := make(map[string][]string)
	var groupOrder []string

	for _, svcName := range serviceNames {
		group := cfg.Services[svcName].Group
		if group == "" {
			ungrouped = append(ungrouped, svcName)
		} else {
			if _, seen := groupServices[group]; !seen {
				groupOrder = append(groupOrder, group)
			}
			groupServices[group] = append(groupServices[group], svcName)
		}
	}
	sort.Strings(groupOrder)

	for _, svcName := range ungrouped {
		printServiceLine(w, cfg, svcName, ports[svcName], check)
	}
	if len(ungrouped) > 0 && len(groupOrder) > 0 {
		lipgloss.Fprintln(w)
	}

	for i, group := range groupOrder {
		lipgloss.Fprintln(w, "  "+ui.GroupStyle.Render(group))
		for _, svcName := range groupServices[group] {
			printServiceLine(w, cfg, svcName, ports[svcName], check)
		}
		if i < len(groupOrder)-1 {
			lipgloss.Fprintln(w)
		}
	}
}

func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, check bool) {
	for _, svcName := range serviceNames {
		printServiceLine(w, cfg, svcName, ports[svcName], check)
	}
}

func printServiceLine(w io.Writer, cfg *config.Config, svcName string, port int, check bool) {
	svc := cfg.Services[svcName]
	portDisplay := ui.PortStyle.Render(fmt.Sprintf("%d", port))
	if url := serviceURL(svc.Protocol, port); url != "" {
		portDisplay = ui.UrlStyle.Render(url)
	}

	status := ""
	if check {
		if portcheck.IsUp(port) {
			status = "  " + ui.StatusUp
		} else {
			status = "  " + ui.StatusDown
		}
	}

	line := fmt.Sprintf("    %s  %s  %s %s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", svc.EnvVar)),
		ui.Arrow,
		portDisplay,
		status,
	)
	lipgloss.Fprintln(w, line)
}

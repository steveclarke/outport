package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var forceFlag bool

var applyCmd = &cobra.Command{
	Use:     "apply",
	Aliases: []string{"a"},
	Short:   "Apply port configuration and write .env files",
	Long:    "Reads .outport.yml, allocates deterministic ports, saves to the central registry, and writes them to .env files.",
	RunE:    runApply,
}

func init() {
	applyCmd.Flags().BoolVar(&forceFlag, "force", false, "ignore existing allocations and re-allocate all ports")
	rootCmd.AddCommand(applyCmd)

	// Hidden backward-compat aliases
	for _, alias := range []string{"up", "register"} {
		aliasCmd := &cobra.Command{
			Use:    alias,
			Hidden: true,
			RunE:   runApply,
		}
		aliasCmd.Flags().BoolVar(&forceFlag, "force", false, "")
		rootCmd.AddCommand(aliasCmd)
	}
}

func runApply(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	dir, cfg, wt, reg := ctx.Dir, ctx.Cfg, ctx.WT, ctx.Reg

	existing, hasExisting := reg.Get(cfg.Name, wt.Instance)
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
		if old, ok := reg.Get(cfg.Name, wt.Instance); ok {
			for _, port := range old.Ports {
				delete(usedPorts, port)
			}
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

	// Resolve derived values and add to envFileVars
	resolvedDerived := resolveDerivedFromAlloc(cfg, ports)
	for name, fileValues := range resolvedDerived {
		for file, value := range fileValues {
			if envFileVars[file] == nil {
				envFileVars[file] = make(map[string]string)
			}
			envFileVars[file][name] = value
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
		return printApplyJSON(cmd, cfg, wt, ports, resolvedDerived, envFiles)
	}
	return printApplyStyled(cmd, cfg, wt, serviceNames, ports, resolvedDerived, envFiles)
}

// buildEnvVarPorts maps env_var names to allocated port numbers.
func buildEnvVarPorts(cfg *config.Config, ports map[string]int) map[string]int {
	envVarPorts := make(map[string]int)
	for svcName, svc := range cfg.Services {
		if port, ok := ports[svcName]; ok {
			envVarPorts[svc.EnvVar] = port
		}
	}
	return envVarPorts
}

// resolveDerivedFromAlloc resolves derived value templates using allocated ports.
// Returns name → file → resolved value.
func resolveDerivedFromAlloc(cfg *config.Config, ports map[string]int) map[string]map[string]string {
	if len(cfg.Derived) == 0 {
		return nil
	}
	envVarPorts := buildEnvVarPorts(cfg, ports)
	return config.ResolveDerived(cfg.Derived, envVarPorts)
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
	Up            *bool    `json:"up,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

type derivedJSON struct {
	Value    string            `json:"value,omitempty"`     // when all files share a value
	EnvFiles []string          `json:"env_files,omitempty"` // when all files share a value
	Values   map[string]string `json:"values,omitempty"`    // file → value when per-file
}

type applyJSON struct {
	Project  string                 `json:"project"`
	Instance string                 `json:"instance"`
	Services map[string]svcJSON     `json:"services"`
	Derived  map[string]derivedJSON `json:"derived,omitempty"`
	EnvFiles []string               `json:"env_files"`
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

func printApplyJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, ports map[string]int, resolvedDerived map[string]map[string]string, envFiles []string) error {
	out := applyJSON{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: buildServiceMap(cfg, ports),
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

func printHeader(w io.Writer, projectName string, wt *worktree.Info) {
	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}
	header := ui.ProjectStyle.Render(projectName) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)
}

func printApplyStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, serviceNames []string, ports map[string]int, resolvedDerived map[string]map[string]string, envFiles []string) error {
	w := cmd.OutOrStdout()

	printHeader(w, cfg.Name, wt)

	printFlatServices(w, cfg, serviceNames, ports, nil)

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

func printFlatServices(w io.Writer, cfg *config.Config, serviceNames []string, ports map[string]int, portStatus map[int]bool) {
	for _, svcName := range serviceNames {
		printServiceLine(w, cfg, svcName, ports[svcName], portStatus)
	}
}

func printServiceLine(w io.Writer, cfg *config.Config, svcName string, port int, portStatus map[int]bool) {
	svc := cfg.Services[svcName]

	status := ""
	if portStatus != nil {
		if portStatus[port] {
			status = "  " + ui.StatusUp
		} else {
			status = "  " + ui.StatusDown
		}
	}

	url := ""
	if u := serviceURL(svc.Protocol, port); u != "" {
		url = "  " + ui.UrlStyle.Render(u)
	}

	line := fmt.Sprintf("    %s  %s  %s %-5s%s%s",
		ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
		ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", svc.EnvVar)),
		ui.Arrow,
		ui.PortStyle.Render(fmt.Sprintf("%d", port)),
		status,
		url,
	)
	lipgloss.Fprintln(w, line)
}

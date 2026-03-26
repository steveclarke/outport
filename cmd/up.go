package cmd

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/allocation"
	"github.com/steveclarke/outport/internal/allocator"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/envpath"
	"github.com/steveclarke/outport/internal/platform"
	"github.com/steveclarke/outport/internal/portcheck"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/ui"
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
	serviceNames := slices.Sorted(maps.Keys(cfg.Services))

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
	alloc := allocation.Build(cfg, ctx.Instance, dir, ports)

	// Check hostname uniqueness across registry
	selfKey := registry.Key(cfg.Name, ctx.Instance)
	for svcName, hostname := range alloc.Hostnames {
		if conflictKey, found := reg.FindHostname(hostname, selfKey); found {
			return fmt.Errorf("hostname %q (service %q) conflicts with %s", hostname, svcName, conflictKey)
		}
	}

	reg.Set(cfg.Name, ctx.Instance, alloc)

	httpsEnabled := certmanager.IsCAInstalled()

	// Get approved paths from existing allocation; clear if --force.
	var approvedPaths []string
	if !forceFlag && hasExisting {
		approvedPaths = existing.ApprovedExternalFiles
	}

	result, err := writeEnvFiles(dir, cfg, ctx.Instance, ports, alloc.Hostnames, httpsEnabled, EnvWriteOptions{
		AutoApprove:   yesFlag,
		ApprovedPaths: approvedPaths,
		Aliases:       alloc.Aliases,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
	})
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
	return slices.Sorted(maps.Keys(files))
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
	return writeJSON(cmd, out)
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


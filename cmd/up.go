// cmd/up.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/outport-app/outport/internal/allocator"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/worktree"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Allocate ports and write to .env",
	Long:  "Reads .outport.yml, allocates deterministic ports, and writes them to the project's .env file.",
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

	// 1. Load config
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}

	// 2. Detect worktree
	wt, err := worktree.Detect(dir)
	if err != nil {
		return fmt.Errorf("detecting worktree: %w", err)
	}

	// 3. Load registry
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		return err
	}

	// 4. Check for existing allocation
	existing, hasExisting := reg.Get(cfg.Name, wt.Instance)

	// 5. Build used ports set (excluding our own existing allocation)
	usedPorts := reg.UsedPorts()
	if hasExisting {
		for _, port := range existing.Ports {
			delete(usedPorts, port)
		}
	}

	// 6. Build port allocations — reuse existing where possible, allocate new ones
	ports := make(map[string]int)
	envVars := make(map[string]string)

	// Sort service names for deterministic allocation order
	serviceNames := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		// Reuse existing port if this service was already allocated
		if hasExisting {
			if existingPort, ok := existing.Ports[svcName]; ok {
				ports[svcName] = existingPort
				usedPorts[existingPort] = true
				envVars[svc.EnvVar] = fmt.Sprintf("%d", existingPort)
				continue
			}
		}
		// New service — allocate a port
		port, err := allocator.Allocate(cfg.Name, wt.Instance, svcName, usedPorts)
		if err != nil {
			return fmt.Errorf("allocating port for %s: %w", svcName, err)
		}
		ports[svcName] = port
		usedPorts[port] = true
		envVars[svc.EnvVar] = fmt.Sprintf("%d", port)
	}

	// 7. Save to registry
	reg.Set(cfg.Name, wt.Instance, registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
	})
	if err := reg.Save(); err != nil {
		return err
	}

	// 8. Merge into .env
	envPath := filepath.Join(dir, ".env")
	if err := dotenv.Merge(envPath, envVars); err != nil {
		return err
	}

	// 9. Print summary
	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "outport: %s [%s]\n", cfg.Name, instance)
	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) → %d\n", svcName, svc.EnvVar, ports[svcName])
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nPorts written to .env\n")

	return nil
}

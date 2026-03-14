package cmd

import (
	"encoding/json"
	"fmt"
	"os"
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
	envVars := make(map[string]string)

	serviceNames := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		if hasExisting {
			if existingPort, ok := existing.Ports[svcName]; ok {
				ports[svcName] = existingPort
				usedPorts[existingPort] = true
				envVars[svc.EnvVar] = fmt.Sprintf("%d", existingPort)
				continue
			}
		}
		port, err := allocator.Allocate(cfg.Name, wt.Instance, svcName, svc.DefaultPort, usedPorts)
		if err != nil {
			return fmt.Errorf("allocating port for %s: %w", svcName, err)
		}
		ports[svcName] = port
		usedPorts[port] = true
		envVars[svc.EnvVar] = fmt.Sprintf("%d", port)
	}

	reg.Set(cfg.Name, wt.Instance, registry.Allocation{
		ProjectDir: dir,
		Ports:      ports,
	})
	if err := reg.Save(); err != nil {
		return err
	}

	envPath := filepath.Join(dir, ".env")
	if err := dotenv.Merge(envPath, envVars); err != nil {
		return err
	}

	// Output
	if jsonFlag {
		return printUpJSON(cmd, cfg, wt, ports)
	}
	return printUpStyled(cmd, cfg, wt, serviceNames, ports)
}

type upOutput struct {
	Project  string         `json:"project"`
	Instance string         `json:"instance"`
	Services map[string]int `json:"services"`
}

func printUpJSON(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, ports map[string]int) error {
	out := upOutput{
		Project:  cfg.Name,
		Instance: wt.Instance,
		Services: ports,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printUpStyled(cmd *cobra.Command, cfg *config.Config, wt *worktree.Info, serviceNames []string, ports map[string]int) error {
	w := cmd.OutOrStdout()

	instance := wt.Instance
	if wt.IsWorktree {
		instance += " (worktree)"
	}

	header := ui.ProjectStyle.Render(cfg.Name) + " " + ui.InstanceStyle.Render("["+instance+"]")
	lipgloss.Fprintln(w, header)
	lipgloss.Fprintln(w)

	for _, svcName := range serviceNames {
		svc := cfg.Services[svcName]
		line := fmt.Sprintf("  %s  %s  %s %s",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", svcName)),
			ui.EnvVarStyle.Render(fmt.Sprintf("%-20s", svc.EnvVar)),
			ui.Arrow,
			ui.PortStyle.Render(fmt.Sprintf("%d", ports[svcName])),
		)
		lipgloss.Fprintln(w, line)
	}

	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.SuccessStyle.Render("Ports written to .env"))
	return nil
}

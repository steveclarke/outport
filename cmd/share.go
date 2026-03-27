package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/config"
	"github.com/steveclarke/outport/internal/envpath"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/settings"
	"github.com/steveclarke/outport/internal/tunnel"
	"github.com/steveclarke/outport/internal/tunnel/cloudflare"
	"github.com/steveclarke/outport/internal/ui"
)

var shareCmd = &cobra.Command{
	Use:     "share [service...]",
	Short:   "Tunnel HTTP services to public URLs",
	Long:    "Creates public tunnel URLs for HTTP services using Cloudflare quick tunnels. Each hostname (primary and aliases) gets its own tunnel through the local proxy, so Host headers are correctly rewritten. Shares all HTTP services by default, or specify service names to share specific ones.",
	GroupID: "project",
	Args:    cobra.ArbitraryArgs,
	RunE:    runShare,
}

func init() {
	rootCmd.AddCommand(shareCmd)
}

// tunnelTarget represents a single hostname that needs a tunnel.
// The Label is used as the key in the tunnel manager (and appears in output),
// while TestHostname is the .test hostname the tunnel maps to.
type tunnelTarget struct {
	Label        string // service name for primaries, "service/alias/key" for aliases
	Service      string // the service name this target belongs to
	TestHostname string // the .test hostname (e.g., "myapp.test")
}

func runShare(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("No ports allocated. Run 'outport up' first.")
	}

	services, err := resolveShareServices(ctx, args)
	if err != nil {
		return err
	}

	provider := cloudflare.New()
	if err := provider.CheckAvailable(); err != nil {
		return err
	}

	// Load max_tunnels setting
	cfg, err := settings.Load()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}
	maxTunnels := cfg.Tunnels.Max

	// Build ordered list of tunnel targets: primaries first, then aliases.
	// Each hostname gets its own tunnel through the proxy (port 80).
	var targets []tunnelTarget
	for _, svcName := range services {
		svc := ctx.Cfg.Services[svcName]
		targets = append(targets, tunnelTarget{
			Label:        svcName,
			Service:      svcName,
			TestHostname: alloc.Hostnames[svcName],
		})
		// Add aliases in sorted order for determinism
		if len(svc.Aliases) > 0 && alloc.Aliases[svcName] != nil {
			var aliasKeys []string
			for key := range svc.Aliases {
				aliasKeys = append(aliasKeys, key)
			}
			sort.Strings(aliasKeys)
			for _, key := range aliasKeys {
				aliasHostname, ok := alloc.Aliases[svcName][key]
				if !ok {
					continue
				}
				targets = append(targets, tunnelTarget{
					Label:        svcName + "/alias/" + key,
					Service:      svcName,
					TestHostname: aliasHostname,
				})
			}
		}
	}

	// Apply max_tunnels cap
	var skippedTargets []tunnelTarget
	if len(targets) > maxTunnels {
		skippedTargets = targets[maxTunnels:]
		targets = targets[:maxTunnels]
	}

	mgr := tunnel.NewManager(provider, 15*time.Second)

	// All tunnels point to port 80 (the proxy)
	svcPorts := make(map[string]int, len(targets))
	for _, t := range targets {
		svcPorts[t.Label] = 80
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tunnels, err := mgr.StartAll(sigCtx, svcPorts)
	if err != nil {
		return fmt.Errorf("starting tunnels: %w", err)
	}

	httpsEnabled := certmanager.IsCAInstalled()

	defer func() {
		if sp, err := tunnel.DefaultStatePath(); err == nil {
			tunnel.RemoveState(sp)
		}
		mgr.StopAll()
		// Revert .env files to local URLs (best-effort; user can run 'outport up' if this fails)
		if _, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, EnvWriteOptions{
			AutoApprove:   true,
			ApprovedPaths: alloc.ApprovedExternalFiles,
			Aliases:       alloc.Aliases,
			Stderr:        os.Stderr,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to restore .env files: %v\n", err)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), ui.SuccessStyle.Render("Restored .env files to local URLs."))
		fmt.Fprintln(cmd.OutOrStdout(), ui.DimStyle.Render("Restart your services to revert to local development."))
	}()

	// Index tunnels by label for lookup
	tunnelByLabel := make(map[string]*tunnel.Tunnel, len(tunnels))
	for _, tun := range tunnels {
		tunnelByLabel[tun.Service] = tun
	}

	// Build tunnelURLs (label → URL for .env template vars) and hostnameMap
	// (tunnel hostname → .test hostname for daemon HostOverride routes).
	tunnelURLs := make(map[string]string, len(targets))
	hostnameMap := make(map[string]string, len(targets))

	// Build display rows sorted by target order (primaries first, then aliases)
	var rows []shareDisplayRow

	for _, t := range targets {
		tun, ok := tunnelByLabel[t.Label]
		if !ok {
			continue
		}
		tunnelURLs[t.Label] = tun.URL

		// Extract hostname from tunnel URL for the hostname map
		if parsed, err := url.Parse(tun.URL); err == nil {
			hostnameMap[parsed.Hostname()] = t.TestHostname
		}

		rows = append(rows, shareDisplayRow{
			Service:      t.Service,
			URL:          tun.URL,
			TestHostname: t.TestHostname,
		})
	}

	// Write tunnel state for dashboard and daemon HostOverride routes
	statePath, stateErr := tunnel.DefaultStatePath()
	if stateErr == nil {
		key := registry.Key(ctx.Cfg.Name, ctx.Instance)
		_ = tunnel.WriteState(statePath, key, tunnelURLs, hostnameMap) // best-effort
	}

	result, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, EnvWriteOptions{
		AutoApprove:   yesFlag,
		ApprovedPaths: alloc.ApprovedExternalFiles,
		TunnelURLs:    tunnelURLs,
		Aliases:       alloc.Aliases,
		Stdin:         os.Stdin,
		Stderr:        os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("writing tunnel URLs to .env: %w", err)
	}

	if len(result.NewlyApproved) > 0 {
		alloc.ApprovedExternalFiles = mergeApprovedPaths(alloc.ApprovedExternalFiles, result.NewlyApproved)
		ctx.Reg.Set(ctx.Cfg.Name, ctx.Instance, alloc)
		if err := ctx.Reg.Save(); err != nil {
			return err
		}
	}

	if jsonFlag {
		if err := printShareJSON(cmd, tunnels, ctx.Cfg, result.ResolvedComputed, result.ExternalFiles); err != nil {
			return err
		}
	} else {
		printShareStyled(cmd, rows)
		if len(skippedTargets) > 0 {
			var skippedHostnames []string
			for _, t := range skippedTargets {
				skippedHostnames = append(skippedHostnames, t.TestHostname)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: tunnel limit reached (%d). Skipped: %s\n",
				maxTunnels, strings.Join(skippedHostnames, ", "))
		}
		printExternalFilesWarning(cmd.OutOrStdout(), result.ExternalFiles)
	}

	// Block until signal
	<-sigCtx.Done()
	return nil
}

// resolveShareServices returns the sorted list of service names to share.
func resolveShareServices(ctx *projectContext, args []string) ([]string, error) {
	if len(args) > 0 {
		// Validate named services
		for _, name := range args {
			svc, ok := ctx.Cfg.Services[name]
			if !ok {
				return nil, FlagErrorf("unknown service %q", name)
			}
			if svc.Hostname == "" {
				return nil, FlagErrorf("service %q has no hostname and cannot be shared", name)
			}
		}
		sort.Strings(args)
		return args, nil
	}

	// Default: all web services (those with a hostname)
	var services []string
	for name, svc := range ctx.Cfg.Services {
		if svc.Hostname != "" {
			services = append(services, name)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no shareable services found. Add 'hostname' to a service in outport.yml")
	}
	sort.Strings(services)
	return services, nil
}

// JSON output types

type tunnelJSON struct {
	Service string `json:"service"`
	URL     string `json:"url"`
	Port    int    `json:"port"`
}

type shareJSON struct {
	Tunnels       []tunnelJSON            `json:"tunnels"`
	Computed      map[string]computedJSON `json:"computed,omitempty"`
	ExternalFiles []externalFileJSON      `json:"external_files,omitempty"`
}

func printShareJSON(cmd *cobra.Command, tunnels []*tunnel.Tunnel, cfg *config.Config, resolvedComputed map[string]map[string]string, externalFiles []envpath.EnvFilePath) error {
	out := shareJSON{}
	for _, tun := range tunnels {
		out.Tunnels = append(out.Tunnels, tunnelJSON{
			Service: tun.Service,
			URL:     tun.URL,
			Port:    tun.Port,
		})
	}
	out.Computed = buildComputedMap(cfg.Computed, resolvedComputed)
	out.ExternalFiles = toExternalFileJSON(externalFiles)
	n := len(out.Tunnels)
	summary := fmt.Sprintf("%d %s shared", n, pluralize(n, "tunnel", "tunnels"))
	return writeJSON(cmd, out, summary)
}

type shareDisplayRow struct {
	Service      string
	URL          string
	TestHostname string
}

func printShareStyled(cmd *cobra.Command, rows []shareDisplayRow) {
	w := cmd.OutOrStdout()

	lipgloss.Fprintln(w, fmt.Sprintf("Sharing %d %s:",
		len(rows), pluralize(len(rows), "URL", "URLs")))
	lipgloss.Fprintln(w)

	for _, row := range rows {
		line := fmt.Sprintf("  %s  %s %s %s",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", row.Service)),
			ui.UrlStyle.Render(row.URL),
			ui.Arrow,
			row.TestHostname,
		)
		lipgloss.Fprintln(w, line)
	}

	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.SuccessStyle.Render("Updated .env files with tunnel URLs."))
	lipgloss.Fprintln(w, ui.DimStyle.Render("Restart your services to pick up the new URLs."))
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("Press Ctrl+C to stop sharing."))
}


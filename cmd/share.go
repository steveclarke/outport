package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/envpath"
	"github.com/outport-app/outport/internal/tunnel"
	"github.com/outport-app/outport/internal/tunnel/cloudflare"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:     "share [service...]",
	Short:   "Tunnel HTTP services to public URLs",
	Long:    "Creates public tunnel URLs for HTTP services using Cloudflare quick tunnels. Shares all HTTP services by default, or specify service names to share specific ones.",
	GroupID: "project",
	Args:    cobra.ArbitraryArgs,
	RunE:    runShare,
}

func init() {
	rootCmd.AddCommand(shareCmd)
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

	mgr := tunnel.NewManager(provider, 15*time.Second)

	// Build service→port map
	svcPorts := make(map[string]int)
	for _, name := range services {
		svcPorts[name] = alloc.Ports[name]
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tunnels, err := mgr.StartAll(sigCtx, svcPorts)
	if err != nil {
		return fmt.Errorf("starting tunnels: %w", err)
	}

	httpsEnabled := certmanager.IsCAInstalled()

	defer func() {
		mgr.StopAll()
		// Revert .env files to local URLs (best-effort; user can run 'outport up' if this fails)
		if _, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, nil,
			true, alloc.ApprovedExternalFiles, nil, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to restore .env files: %v\n", err)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), ui.SuccessStyle.Render("Restored .env files to local URLs."))
		fmt.Fprintln(cmd.OutOrStdout(), ui.DimStyle.Render("Restart your services to revert to local development."))
	}()

	// Sort once for deterministic output in both modes
	sort.Slice(tunnels, func(i, j int) bool {
		return tunnels[i].Service < tunnels[j].Service
	})

	// Build tunnel URL map and rewrite .env files
	tunnelURLs := make(map[string]string)
	for _, tun := range tunnels {
		tunnelURLs[tun.Service] = tun.URL
	}

	result, err := writeEnvFiles(ctx.Dir, ctx.Cfg, ctx.Instance, alloc.Ports, alloc.Hostnames, httpsEnabled, tunnelURLs,
		yesFlag, alloc.ApprovedExternalFiles, os.Stdin, os.Stderr)
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
		printShareStyled(cmd, tunnels)
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
			if svc.Protocol != "http" && svc.Protocol != "https" {
				return nil, FlagErrorf("service %q does not have an HTTP protocol and cannot be shared", name)
			}
		}
		sort.Strings(args)
		return args, nil
	}

	// Default: all HTTP services
	var services []string
	for name, svc := range ctx.Cfg.Services {
		if svc.Protocol == "http" || svc.Protocol == "https" {
			services = append(services, name)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no shareable services found. Add 'protocol: http' to a service in outport.yml")
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
	return writeJSON(cmd, out)
}

func printShareStyled(cmd *cobra.Command, tunnels []*tunnel.Tunnel) {
	w := cmd.OutOrStdout()

	lipgloss.Fprintln(w, fmt.Sprintf("Sharing %d %s:",
		len(tunnels), pluralize(len(tunnels), "service", "services")))
	lipgloss.Fprintln(w)

	for _, tun := range tunnels {
		line := fmt.Sprintf("  %s  %s %s localhost:%d",
			ui.ServiceStyle.Render(fmt.Sprintf("%-16s", tun.Service)),
			ui.UrlStyle.Render(tun.URL),
			ui.Arrow,
			tun.Port,
		)
		lipgloss.Fprintln(w, line)
	}

	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.SuccessStyle.Render("Updated .env files with tunnel URLs."))
	lipgloss.Fprintln(w, ui.DimStyle.Render("Restart your services to pick up the new URLs."))
	lipgloss.Fprintln(w)
	lipgloss.Fprintln(w, ui.DimStyle.Render("Press Ctrl+C to stop sharing."))
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

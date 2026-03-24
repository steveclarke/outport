package cmd

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/steveclarke/outport/internal/lanip"
	"github.com/steveclarke/outport/internal/qrcode"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/tunnel"
	"github.com/steveclarke/outport/internal/ui"
	"github.com/spf13/cobra"
)

var qrTunnelFlag bool
var qrInterfaceFlag string

var qrCmd = &cobra.Command{
	Use:     "qr [service]",
	Short:   "Show QR codes for accessing services from mobile devices",
	Long:    "Displays a QR code encoding a LAN URL (IP + port) for HTTP services. Scan with your phone to open the dev app. Use --tunnel to show tunnel URLs instead (requires active outport share).",
	GroupID: "project",
	Args:    MaximumArgs(1, "accepts at most one service name"),
	RunE:    runQR,
}

func init() {
	qrCmd.Flags().BoolVar(&qrTunnelFlag, "tunnel", false, "show tunnel URL instead of LAN URL (requires active outport share)")
	qrCmd.Flags().StringVar(&qrInterfaceFlag, "interface", "", "network interface for LAN IP detection (e.g., en0, en1)")
	rootCmd.AddCommand(qrCmd)
}

func runQR(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}

	alloc, ok := ctx.Reg.Get(ctx.Cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("No ports allocated. Run 'outport up' first.")
	}

	services := resolveHTTPServices(ctx, alloc)

	if len(args) == 1 {
		name := args[0]
		if _, ok := ctx.Cfg.Services[name]; !ok {
			return fmt.Errorf("Service %q not found in outport.yml.", name)
		}
		port, ok := alloc.Ports[name]
		if !ok {
			return fmt.Errorf("No port allocated for %q. Run 'outport up' first.", name)
		}
		protocol := ctx.Cfg.Services[name].Protocol
		if protocol != "http" && protocol != "https" {
			return fmt.Errorf("Service %q has no HTTP protocol. QR codes are only available for HTTP services.", name)
		}
		services = map[string]int{name: port}
	}

	if len(services) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No HTTP services found. Add 'protocol: http' to services in outport.yml.")
		return nil
	}

	if qrTunnelFlag {
		return printTunnelQR(cmd, ctx, services)
	}
	return printLANQR(cmd, services)
}

func printLANQR(cmd *cobra.Command, services map[string]int) error {
	ip, err := lanip.Detect(qrInterfaceFlag)
	if err != nil {
		return fmt.Errorf("detecting LAN IP: %w", err)
	}

	if jsonFlag {
		return printQRJSON(cmd, ip, services, nil)
	}

	w := cmd.OutOrStdout()
	for _, name := range slices.Sorted(maps.Keys(services)) {
		port := services[name]
		url := formatLANURL(ip, port)

		lipgloss.Fprintln(w, ui.ServiceStyle.Render(name))
		qr, qrErr := qrcode.Terminal(url)
		if qrErr != nil {
			return fmt.Errorf("generating QR for %s: %w", name, qrErr)
		}
		fmt.Fprint(w, qr)
		lipgloss.Fprintln(w, ui.UrlStyle.Render(url))
		lipgloss.Fprintln(w, ui.DimStyle.Render("Scan with your phone \u00b7 same Wi-Fi network"))
		if !probeLAN(ip, port) {
			lipgloss.Fprintln(w, ui.DimStyle.Render("Service may be bound to localhost only. Bind to 0.0.0.0 to access from other devices."))
		}
		lipgloss.Fprintln(w)
	}
	return nil
}

func printTunnelQR(cmd *cobra.Command, ctx *projectContext, services map[string]int) error {
	statePath, err := tunnel.DefaultStatePath()
	if err != nil {
		return fmt.Errorf("resolving tunnel state path: %w", err)
	}

	state, err := tunnel.ReadState(statePath)
	if err != nil {
		return fmt.Errorf("reading tunnel state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("No active tunnels. Run 'outport share' first.")
	}

	key := registry.Key(ctx.Cfg.Name, ctx.Instance)
	tunnelURLs, ok := state.Tunnels[key]
	if !ok {
		return fmt.Errorf("No tunnels active for %s/%s. Run 'outport share' first.", ctx.Cfg.Name, ctx.Instance)
	}

	if jsonFlag {
		return printQRJSON(cmd, nil, services, tunnelURLs)
	}

	w := cmd.OutOrStdout()
	for _, name := range slices.Sorted(maps.Keys(services)) {
		url, ok := tunnelURLs[name]
		if !ok {
			continue
		}

		lipgloss.Fprintln(w, ui.ServiceStyle.Render(name))
		qr, qrErr := qrcode.Terminal(url)
		if qrErr != nil {
			return fmt.Errorf("generating QR for %s: %w", name, qrErr)
		}
		fmt.Fprint(w, qr)
		lipgloss.Fprintln(w, ui.UrlStyle.Render(url))
		lipgloss.Fprintln(w, ui.DimStyle.Render("Scan with your phone \u00b7 works from any network"))
		lipgloss.Fprintln(w)
	}
	return nil
}

// resolveHTTPServices returns a map of service name -> port for all HTTP services.
func resolveHTTPServices(ctx *projectContext, alloc registry.Allocation) map[string]int {
	services := make(map[string]int)
	for name, svc := range ctx.Cfg.Services {
		if svc.Protocol == "http" || svc.Protocol == "https" {
			if port, ok := alloc.Ports[name]; ok {
				services[name] = port
			}
		}
	}
	return services
}

// qrServiceJSON is the JSON output for a single service's QR URLs.
type qrServiceJSON struct {
	Service   string `json:"service"`
	LANURL    string `json:"lan_url,omitempty"`
	TunnelURL string `json:"tunnel_url,omitempty"`
	Port      int    `json:"port"`
}

func printQRJSON(cmd *cobra.Command, ip net.IP, services map[string]int, tunnelURLs map[string]string) error {
	var out []qrServiceJSON
	for _, name := range slices.Sorted(maps.Keys(services)) {
		port := services[name]
		svc := qrServiceJSON{
			Service: name,
			Port:    port,
		}
		if ip != nil {
			svc.LANURL = formatLANURL(ip, port)
		}
		if tunnelURLs != nil {
			svc.TunnelURL = tunnelURLs[name]
		}
		out = append(out, svc)
	}
	return writeJSON(cmd, out)
}

func formatLANURL(ip net.IP, port int) string {
	return "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port))
}

func probeLAN(ip net.IP, port int) bool {
	addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

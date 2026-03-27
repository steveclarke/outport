package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/steveclarke/outport/internal/certmanager"
	"github.com/steveclarke/outport/internal/daemon"
	"github.com/steveclarke/outport/internal/registry"
	"github.com/steveclarke/outport/internal/settings"
	"github.com/spf13/cobra"
)

var daemonPort int

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the DNS and proxy daemon (invoked by launchd)",
	Hidden: true,
	RunE:   runDaemon,
}

func init() {
	daemonCmd.Flags().IntVar(&daemonPort, "port", 80, "HTTP proxy listen port")
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	regPath, err := registry.DefaultPath()
	if err != nil {
		return err
	}

	s, err := settings.Load()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	cfg := &daemon.DaemonConfig{
		DNSAddr:          "127.0.0.1:15353",
		RegistryPath:     regPath,
		Version:          version,
		DNSTTL:           uint32(s.DNS.TTL), // #nosec G115 -- TTL is a small config value, no overflow risk
		HealthInterval:   s.Dashboard.HealthInterval,
		NetworkInterface: s.Network.Interface,
	}

	// Try launchd HTTP socket activation (darwin only)
	if httpLn, err := activateLaunchdHTTPSocket(); err == nil && httpLn != nil {
		cfg.HTTPListener = httpLn
		cfg.ProxyAddr = httpLn.Addr().String()
	} else {
		cfg.ProxyAddr = fmt.Sprintf(":%d", daemonPort)
	}

	// Try launchd HTTPS socket activation (darwin only)
	if httpsLn, err := activateLaunchdHTTPSSocket(); err == nil && httpsLn != nil {
		cfg.HTTPSListener = httpsLn
	}

	// Wire TLS if the CA is installed
	if certmanager.IsCAInstalled() {
		caCertPath, caKeyPath, err := certmanager.CAPaths()
		if err != nil {
			return fmt.Errorf("resolving CA paths: %w", err)
		}
		cacheDir, err := certmanager.CertCacheDir()
		if err != nil {
			return fmt.Errorf("resolving cert cache dir: %w", err)
		}

		store, err := certmanager.NewCertStore(caCertPath, caKeyPath, cacheDir)
		if err != nil {
			return fmt.Errorf("initializing cert store: %w", err)
		}

		cfg.TLSConfig = &tls.Config{
			GetCertificate: store.GetCertificate,
		}
	}

	d, err := daemon.New(cfg)
	if err != nil {
		return fmt.Errorf("creating daemon: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return d.Run(ctx)
}

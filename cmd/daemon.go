package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/outport-app/outport/internal/daemon"
	"github.com/outport-app/outport/internal/registry"
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

	cfg := &daemon.DaemonConfig{
		DNSAddr:      "127.0.0.1:15353",
		ProxyAddr:    fmt.Sprintf(":%d", daemonPort),
		RegistryPath: regPath,
	}

	d, err := daemon.New(cfg)
	if err != nil {
		return fmt.Errorf("creating daemon: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return d.Run(ctx)
}

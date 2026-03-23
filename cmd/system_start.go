package cmd

import (
	"fmt"
	"os"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/portcheck"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var systemStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the outport system",
	Long:  "Starts the outport daemon. On first run, installs the .test DNS resolver, generates a local Certificate Authority, and adds it to the system trust store.",
	Args:  NoArgs,
	RunE:  runSystemStart,
}

var systemUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove outport system components",
	Long:  "Unloads the daemon, removes the LaunchAgent, DNS resolver, CA certificate, and cached server certs.",
	Args:  NoArgs,
	RunE:  runSystemUninstall,
}

func init() {
	systemCmd.AddCommand(systemStartCmd)
	systemCmd.AddCommand(systemUninstallCmd)
}

func runSystemStart(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// Already set up — just ensure the agent is running
	if platform.IsSetup() {
		if platform.IsAgentLoaded() {
			if jsonFlag {
				return printSystemStatusJSON(w, "already_running")
			}
			fmt.Fprintln(w, "Outport system is already running.")
			return nil
		}

		if portcheck.IsListening(80) {
			return fmt.Errorf("port 80 is already in use — stop the other server first")
		}
		if portcheck.IsListening(443) {
			return fmt.Errorf("port 443 is already in use — stop the other server first")
		}

		if err := platform.LoadAgent(); err != nil {
			return err
		}

		if jsonFlag {
			return printSystemStatusJSON(w, "started")
		}

		fmt.Fprintln(w, ui.SuccessStyle.Render("Outport system started."))
		return nil
	}

	// First-time setup
	if portcheck.IsListening(80) {
		return fmt.Errorf("port 80 is already in use — stop the other server first")
	}
	if portcheck.IsListening(443) {
		return fmt.Errorf("port 443 is already in use — stop the other server first")
	}

	caGenerated := false
	caTrusted := false

	if !jsonFlag {
		fmt.Fprintln(w, "Installing .test domain routing with HTTPS...")
		fmt.Fprintln(w)
	}

	if err := resolveAndWritePlist(); err != nil {
		return err
	}

	if !jsonFlag {
		fmt.Fprintln(w, "  Your password is needed to configure .test DNS resolution.")
		fmt.Fprintln(w)
	}
	if err := platform.WriteResolverFile(); err != nil {
		return err
	}

	caCertPath, caKeyPath, err := certmanager.CAPaths()
	if err != nil {
		return err
	}

	if !certmanager.IsCAInstalled() {
		if !jsonFlag {
			fmt.Fprintln(w, "  Generating local Certificate Authority...")
		}
		if err := certmanager.GenerateCA(caCertPath, caKeyPath); err != nil {
			return err
		}
		caGenerated = true
	}

	if !jsonFlag {
		fmt.Fprintln(w, "  Adding CA to system trust store (you may see a password dialog)...")
	}
	if err := platform.TrustCA(caCertPath); err != nil {
		return fmt.Errorf("CA must be trusted for HTTPS to work: %w", err)
	}
	caTrusted = true

	if err := platform.LoadAgent(); err != nil {
		return err
	}

	if jsonFlag {
		return printSystemStartJSON(cmd, caGenerated, caTrusted)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Done! *.test domains are now routing with HTTPS."))
	fmt.Fprintln(w, ui.DimStyle.Render("Dashboard: https://outport.test"))
	return nil
}

func runSystemUninstall(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	caRemoved := false
	certsCleaned := false

	if !jsonFlag {
		fmt.Fprintln(w, "Unloading daemon...")
	}
	_ = platform.UnloadAgent()

	if !jsonFlag {
		fmt.Fprintln(w, "Removing LaunchAgent...")
	}
	_ = platform.RemovePlist()

	if !jsonFlag {
		fmt.Fprintln(w, "Removing /etc/resolver/test (sudo may prompt for your password)...")
	}
	if err := platform.RemoveResolverFile(); err != nil {
		return err
	}

	caCertPath, caKeyPath, err := certmanager.CAPaths()
	if err != nil {
		return err
	}
	if certmanager.IsCAInstalled() {
		if !jsonFlag {
			fmt.Fprintln(w, "Removing CA from trust store...")
		}
		_ = platform.UntrustCA(caCertPath)
		certmanager.DeleteCA(caCertPath, caKeyPath)
		caRemoved = true
	}

	if !jsonFlag {
		fmt.Fprintln(w, "Removing cached certificates...")
	}
	if err := certmanager.DeleteCertCache(); err == nil {
		certsCleaned = true
	}

	if !jsonFlag {
		fmt.Fprintln(w, "Removing registry...")
	}
	registryPath, err := registry.DefaultPath()
	if err == nil {
		if err := os.Remove(registryPath); err != nil && !os.IsNotExist(err) && !jsonFlag {
			fmt.Fprintf(w, "  Warning: could not remove registry: %v\n", err)
		}
	}

	if jsonFlag {
		return printSystemUninstallJSON(cmd, caRemoved, certsCleaned)
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Uninstall complete. DNS resolver, daemon, certificates, and registry removed."))
	return nil
}


type systemStartJSON struct {
	CAGenerated bool `json:"ca_generated"`
	CATrusted   bool `json:"ca_trusted"`
}

func printSystemStartJSON(cmd *cobra.Command, caGenerated, caTrusted bool) error {
	return writeJSON(cmd, systemStartJSON{
		CAGenerated: caGenerated,
		CATrusted:   caTrusted,
	})
}

type systemUninstallJSON struct {
	CARemoved    bool `json:"ca_removed"`
	CertsCleaned bool `json:"certs_cleaned"`
}

func printSystemUninstallJSON(cmd *cobra.Command, caRemoved, certsCleaned bool) error {
	return writeJSON(cmd, systemUninstallJSON{
		CARemoved:    caRemoved,
		CertsCleaned: certsCleaned,
	})
}

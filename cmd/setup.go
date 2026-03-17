package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/outport-app/outport/internal/certmanager"
	"github.com/outport-app/outport/internal/platform"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install the DNS resolver, daemon, and local CA for HTTPS",
	Long:  "Installs the .test DNS resolver, LaunchAgent, and local Certificate Authority so that *.test hostnames resolve to your local services with HTTPS.",
	Args:  NoArgs,
	RunE:  runSetup,
}

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Remove the DNS resolver, daemon, and certificates",
	Long:  "Unloads the daemon, removes the LaunchAgent plist, removes the .test DNS resolver file, and removes the CA certificate and cached server certs.",
	Args:  NoArgs,
	RunE:  runTeardown,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(teardownCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if platform.IsSetup() {
		fmt.Fprintln(w, "Already set up. Use 'outport teardown' to remove and re-install.")
		return nil
	}

	if isPortInUse(80) {
		return fmt.Errorf("port 80 is already in use — stop the other server first")
	}
	if isPortInUse(443) {
		return fmt.Errorf("port 443 is already in use — stop the other server first")
	}

	outportBin, err := exec.LookPath("outport")
	if err != nil {
		return fmt.Errorf("could not find outport binary in PATH: %w", err)
	}

	caGenerated := false
	caTrusted := false

	if !jsonFlag {
		fmt.Fprintln(w, "Installing .test domain routing with HTTPS...")
		fmt.Fprintln(w)
	}

	if err := platform.WritePlist(outportBin); err != nil {
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
		return printSetupJSON(cmd, caGenerated, caTrusted)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Done! *.test domains are now routing with HTTPS."))
	return nil
}

func runTeardown(cmd *cobra.Command, args []string) error {
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

	if jsonFlag {
		return printTeardownJSON(cmd, caRemoved, certsCleaned)
	}

	fmt.Fprintln(w, ui.SuccessStyle.Render("Teardown complete. DNS resolver, daemon, and SSL certificates removed."))
	return nil
}

func isPortInUse(port int) bool {
	out, err := exec.Command("lsof", "-iTCP:"+fmt.Sprintf("%d", port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

type setupJSON struct {
	CAGenerated bool `json:"ca_generated"`
	CATrusted   bool `json:"ca_trusted"`
}

func printSetupJSON(cmd *cobra.Command, caGenerated, caTrusted bool) error {
	data, err := json.MarshalIndent(setupJSON{
		CAGenerated: caGenerated,
		CATrusted:   caTrusted,
	}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

type teardownJSON struct {
	CARemoved    bool `json:"ca_removed"`
	CertsCleaned bool `json:"certs_cleaned"`
}

func printTeardownJSON(cmd *cobra.Command, caRemoved, certsCleaned bool) error {
	data, err := json.MarshalIndent(teardownJSON{
		CARemoved:    caRemoved,
		CertsCleaned: certsCleaned,
	}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

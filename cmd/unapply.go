package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/dotenv"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var unapplyCmd = &cobra.Command{
	Use:   "unapply",
	Short: "Remove ports and clean .env files",
	Long:  "Removes the managed block from all .env files and removes the project from the central registry.",
	RunE:  runUnapply,
}

func init() {
	rootCmd.AddCommand(unapplyCmd)
}

func runUnapply(cmd *cobra.Command, args []string) error {
	ctx, err := loadProjectContext()
	if err != nil {
		return err
	}
	cfg, reg := ctx.Cfg, ctx.Reg

	_, ok := reg.Get(cfg.Name, ctx.Instance)
	if !ok {
		return fmt.Errorf("project %q (instance %q) is not registered", cfg.Name, ctx.Instance)
	}

	// Clean managed blocks from .env files
	cleanedFiles := cleanEnvFiles(ctx.Dir, cfg)

	reg.Remove(cfg.Name, ctx.Instance)
	if err := reg.Save(); err != nil {
		return err
	}

	if jsonFlag {
		return printUnapplyJSON(cmd, cfg.Name, ctx.Instance, cleanedFiles)
	}
	return printUnapplyStyled(cmd, cfg.Name, ctx.Instance, cleanedFiles)
}

// cleanEnvFiles removes the outport fenced block from all .env files
// referenced by the config. Returns the list of files that were cleaned.
func cleanEnvFiles(dir string, cfg *config.Config) []string {
	seen := make(map[string]bool)
	for _, svc := range cfg.Services {
		for _, f := range svc.EnvFiles {
			seen[f] = true
		}
	}
	for _, dv := range cfg.Derived {
		for _, f := range dv.EnvFiles {
			seen[f] = true
		}
	}

	var cleaned []string
	for f := range seen {
		envPath := filepath.Join(dir, f)
		if err := dotenv.RemoveBlock(envPath); err == nil {
			cleaned = append(cleaned, f)
		}
	}
	return cleaned
}

func printUnapplyJSON(cmd *cobra.Command, project, instance string, cleanedFiles []string) error {
	out := struct {
		Project      string   `json:"project"`
		Instance     string   `json:"instance"`
		Status       string   `json:"status"`
		CleanedFiles []string `json:"cleaned_files"`
	}{
		Project:      project,
		Instance:     instance,
		Status:       "unapplied",
		CleanedFiles: cleanedFiles,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

func printUnapplyStyled(cmd *cobra.Command, project, instanceName string, cleanedFiles []string) error {
	w := cmd.OutOrStdout()
	printHeader(w, project, instanceName)
	fmt.Fprintln(w, ui.SuccessStyle.Render("Unapplied. All ports freed."))
	if len(cleanedFiles) > 0 {
		fmt.Fprintln(w, ui.SuccessStyle.Render("Cleaned managed variables from:"))
		for _, f := range cleanedFiles {
			fmt.Fprintln(w, ui.SuccessStyle.Render("  "+f))
		}
	}
	return nil
}

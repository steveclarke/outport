package cmd

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/outport-app/outport/internal/config"
	"github.com/outport-app/outport/internal/doctor"
	"github.com/outport-app/outport/internal/registry"
	"github.com/outport-app/outport/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check the health of the outport system",
	Long:    "Runs diagnostic checks on DNS, daemon, certificates, registry, and project configuration. Reports pass/warn/fail for each check with actionable fix suggestions.",
	GroupID: "project",
	Args:    NoArgs,
	RunE:    runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	r := &doctor.Runner{}

	// System checks (always)
	for _, c := range doctor.SystemChecks() {
		r.Add(c)
	}

	// Project checks (when outport.yml found)
	cwd, err := os.Getwd()
	if err == nil {
		if dir, findErr := config.FindDir(cwd); findErr == nil {
			regPath, _ := registry.DefaultPath()
			cfg, configErr := config.Load(dir)
			for _, c := range doctor.ProjectChecks(dir, cfg, configErr, regPath) {
				r.Add(c)
			}
		}
	}

	results := r.Run()

	if jsonFlag {
		if err := printDoctorJSON(cmd, results); err != nil {
			return err
		}
	} else {
		printDoctorStyled(cmd.OutOrStdout(), results)
	}

	if doctor.HasFailures(results) {
		return ErrSilent
	}
	return nil
}

// JSON output

type resultJSON struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

type doctorJSON struct {
	Results []resultJSON `json:"results"`
	Passed  bool         `json:"passed"`
}

func printDoctorJSON(cmd *cobra.Command, results []doctor.Result) error {
	out := doctorJSON{
		Passed: !doctor.HasFailures(results),
	}
	for _, r := range results {
		out.Results = append(out.Results, resultJSON{
			Name:     r.Name,
			Category: r.Category,
			Status:   r.Status.String(),
			Message:  r.Message,
			Fix:      r.Fix,
		})
	}
	return writeJSON(cmd, out)
}

// Styled output

func printDoctorStyled(w io.Writer, results []doctor.Result) {
	currentCategory := ""
	for _, r := range results {
		if r.Category != currentCategory {
			if currentCategory != "" {
				lipgloss.Fprintln(w) // blank line between categories
			}
			lipgloss.Fprintln(w, ui.ProjectStyle.Render(r.Category))
			currentCategory = r.Category
		}

		var icon string
		switch r.Status {
		case doctor.Pass:
			icon = lipgloss.NewStyle().Foreground(ui.Green).Render("✓")
		case doctor.Warn:
			icon = lipgloss.NewStyle().Foreground(ui.Yellow).Render("!")
		case doctor.Fail:
			icon = lipgloss.NewStyle().Foreground(ui.Red).Render("✗")
		}

		lipgloss.Fprintln(w, fmt.Sprintf("  %s %s", icon, r.Message))

		if r.Fix != "" {
			lipgloss.Fprintln(w, fmt.Sprintf("    %s %s", ui.Arrow, ui.DimStyle.Render(r.Fix)))
		}
	}

	lipgloss.Fprintln(w)
	if doctor.HasFailures(results) {
		lipgloss.Fprintln(w, lipgloss.NewStyle().Foreground(ui.Red).Render("Some checks failed. See suggestions above."))
	} else {
		lipgloss.Fprintln(w, ui.SuccessStyle.Render("All checks passed."))
	}
}

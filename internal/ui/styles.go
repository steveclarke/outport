// Package ui defines the terminal color palette and text styles used by all CLI
// command output. It uses the lipgloss library for styled terminal rendering. Every
// command that produces human-readable output references these shared styles to
// ensure a consistent visual identity across the CLI. The styles cover project
// names, instance labels, service names, ports, URLs, hostnames, status indicators,
// and other display elements.
//
// Call Init() once at startup to adapt colors to the terminal environment.
// When NO_COLOR is set (any value, per https://no-color.org), all color is
// stripped but text decoration (bold) is preserved. On dark backgrounds,
// gray tones are shifted brighter so they remain legible.
package ui

import (
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// NOTE: If you add a color or style here, also update initNoColor() and
// the resetStyles() helper in styles_test.go.
var (
	// Brand is the Outport brand accent color (steel blue #2E86AB), used for
	// links and highlighted interactive elements.
	Brand = lipgloss.Color("#2E86AB")

	// Purple is used for project name headings in CLI output.
	Purple = lipgloss.Color("99")

	// Green is used for success messages and the "up" health status indicator.
	Green = lipgloss.Color("42")

	// Gray is used for secondary text like instance labels and env var names.
	Gray = lipgloss.Color("245")

	// LightGray is used for dim/tertiary text and decorative separators.
	LightGray = lipgloss.Color("241")

	// Cyan is used for service names and hostname displays.
	Cyan = lipgloss.Color("86")

	// Yellow is used for port numbers and service URLs to make them stand out.
	Yellow = lipgloss.Color("214")

	// Red is used for error states and the "down" health status indicator.
	Red = lipgloss.Color("196")

	// ProjectStyle renders project names as bold purple text. Used as the
	// top-level heading in commands like `outport status` and `outport list`.
	ProjectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple)

	// InstanceStyle renders instance labels (e.g., "main", "bxcf") in gray.
	// Instance labels appear beneath project names to distinguish between
	// the main checkout and worktree/clone instances.
	InstanceStyle = lipgloss.NewStyle().
			Foreground(Gray)

	// ServiceStyle renders service names (as defined in outport.yml) in cyan.
	ServiceStyle = lipgloss.NewStyle().
			Foreground(Cyan)

	// EnvVarStyle renders environment variable names (e.g., "PORT") in gray.
	EnvVarStyle = lipgloss.NewStyle().
			Foreground(Gray)

	// PortStyle renders port numbers in bold yellow to make them highly visible,
	// since port numbers are one of the primary outputs of most commands.
	PortStyle = lipgloss.NewStyle().
			Foreground(Yellow).
			Bold(true)

	// SuccessStyle renders success messages (e.g., "allocated", "written") in green.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green)

	// DimStyle renders secondary/tertiary text in light gray. Used for
	// supplementary information that should not compete with primary content.
	DimStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	// Arrow is a pre-rendered dim arrow separator ("→") used between related
	// values in output lines (e.g., "PORT → 12345").
	Arrow = DimStyle.Render("→")

	// UrlStyle renders service URLs in yellow to make them clickable and prominent.
	UrlStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	// HostnameStyle renders .test hostnames in cyan, visually grouping them
	// with service names since hostnames derive from service configuration.
	HostnameStyle = lipgloss.NewStyle().
			Foreground(Cyan)

	// StatusUp is a pre-rendered green checkmark with "up" text, displayed
	// by commands that show service health status (e.g., `outport status`).
	StatusUp = lipgloss.NewStyle().Foreground(Green).Render("✓ up")

	// StatusDown is a pre-rendered red X with "down" text, displayed when
	// a service's port is not accepting TCP connections.
	StatusDown = lipgloss.NewStyle().Foreground(Red).Render("✗ down")
)

// noColor is a lipgloss.NoColor{} value, cached to avoid repeated allocation.
var noColor = lipgloss.NoColor{}

// Init detects the terminal environment and adjusts colors accordingly.
// It must be called once at CLI startup before any styled output is rendered.
//
//   - NO_COLOR (any value, including empty): all foreground colors are removed.
//     Bold and other text decoration are preserved per the NO_COLOR spec.
//   - Dark background: Gray and LightGray are shifted brighter (250/247) so
//     secondary text remains legible.
func Init() {
	_, noColorSet := os.LookupEnv("NO_COLOR")

	if noColorSet {
		initNoColor()
	} else if isTerminal(os.Stdin) && lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		initDarkBackground()
	}

	// Re-render pre-rendered strings that depend on style variables.
	Arrow = DimStyle.Render("→")
	StatusUp = lipgloss.NewStyle().Foreground(Green).Render("✓ up")
	StatusDown = lipgloss.NewStyle().Foreground(Red).Render("✗ down")
}

// initNoColor strips all foreground colors from styles, preserving bold/decoration.
func initNoColor() {
	Brand = noColor
	Purple = noColor
	Green = noColor
	Gray = noColor
	LightGray = noColor
	Cyan = noColor
	Yellow = noColor
	Red = noColor

	ProjectStyle = lipgloss.NewStyle().Bold(true)
	InstanceStyle = lipgloss.NewStyle()
	ServiceStyle = lipgloss.NewStyle()
	EnvVarStyle = lipgloss.NewStyle()
	PortStyle = lipgloss.NewStyle().Bold(true)
	SuccessStyle = lipgloss.NewStyle()
	DimStyle = lipgloss.NewStyle()
	UrlStyle = lipgloss.NewStyle()
	HostnameStyle = lipgloss.NewStyle()
}

// isTerminal reports whether f is connected to an interactive terminal.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(f.Fd())
}

// initDarkBackground adjusts grays upward so they are visible on dark terminals.
// The default Gray (245) and LightGray (241) are too dim on dark backgrounds.
func initDarkBackground() {
	Gray = lipgloss.Color("250")
	LightGray = lipgloss.Color("247")

	InstanceStyle = lipgloss.NewStyle().Foreground(Gray)
	EnvVarStyle = lipgloss.NewStyle().Foreground(Gray)
	DimStyle = lipgloss.NewStyle().Foreground(LightGray)
}

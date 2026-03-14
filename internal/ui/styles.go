package ui

import (
	"charm.land/lipgloss/v2"
)

var (
	Purple    = lipgloss.Color("99")
	Green     = lipgloss.Color("42")
	Gray      = lipgloss.Color("245")
	LightGray = lipgloss.Color("241")
	Cyan      = lipgloss.Color("86")
	Yellow    = lipgloss.Color("214")

	// Project header: bold purple
	ProjectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple)

	// Instance label (e.g., "main", "feature-xyz (worktree)")
	InstanceStyle = lipgloss.NewStyle().
			Foreground(Gray)

	// Service name in output
	ServiceStyle = lipgloss.NewStyle().
			Foreground(Cyan)

	// Env var name
	EnvVarStyle = lipgloss.NewStyle().
			Foreground(Gray)

	// Port number
	PortStyle = lipgloss.NewStyle().
			Foreground(Yellow).
			Bold(true)

	// Success message
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green)

	// Dim text for secondary info
	DimStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	// Arrow separator
	Arrow = DimStyle.Render("→")

	// Group header in output
	GroupStyle = lipgloss.NewStyle().
			Foreground(Purple).
			Bold(true)

	// URL for HTTP services
	UrlStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	Red = lipgloss.Color("196")

	// Port status indicators
	StatusUp   = lipgloss.NewStyle().Foreground(Green).Render("✓ up")
	StatusDown = lipgloss.NewStyle().Foreground(Red).Render("✗ down")
)

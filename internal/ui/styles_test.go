package ui

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// resetStyles restores all style variables to their default (light background)
// values so tests don't leak state.
func resetStyles() {
	Brand = lipgloss.Color("#2E86AB")
	Purple = lipgloss.Color("99")
	Green = lipgloss.Color("42")
	Gray = lipgloss.Color("245")
	LightGray = lipgloss.Color("241")
	Cyan = lipgloss.Color("86")
	Yellow = lipgloss.Color("214")
	Red = lipgloss.Color("196")

	ProjectStyle = lipgloss.NewStyle().Bold(true).Foreground(Purple)
	InstanceStyle = lipgloss.NewStyle().Foreground(Gray)
	ServiceStyle = lipgloss.NewStyle().Foreground(Cyan)
	EnvVarStyle = lipgloss.NewStyle().Foreground(Gray)
	PortStyle = lipgloss.NewStyle().Foreground(Yellow).Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(Green)
	DimStyle = lipgloss.NewStyle().Foreground(LightGray)
	UrlStyle = lipgloss.NewStyle().Foreground(Yellow)
	HostnameStyle = lipgloss.NewStyle().Foreground(Cyan)
	Arrow = DimStyle.Render("→")
	StatusUp = lipgloss.NewStyle().Foreground(Green).Render("✓ up")
	StatusDown = lipgloss.NewStyle().Foreground(Red).Render("✗ down")
}

func TestInit_NoColor(t *testing.T) {
	t.Cleanup(resetStyles)
	t.Setenv("NO_COLOR", "")
	Init()

	// All color variables should be NoColor.
	type colorEntry struct {
		name string
		val  any
	}
	colors := []colorEntry{
		{"Brand", Brand},
		{"Purple", Purple},
		{"Green", Green},
		{"Gray", Gray},
		{"LightGray", LightGray},
		{"Cyan", Cyan},
		{"Yellow", Yellow},
		{"Red", Red},
	}
	for _, c := range colors {
		if _, ok := c.val.(lipgloss.NoColor); !ok {
			t.Errorf("%s should be NoColor when NO_COLOR is set, got %T", c.name, c.val)
		}
	}

	// Arrow should be plain text (no ANSI escape codes).
	if Arrow != "→" {
		t.Errorf("Arrow should be plain '→' when NO_COLOR is set, got %q", Arrow)
	}

	// StatusUp/StatusDown should be plain text.
	if StatusUp != "✓ up" {
		t.Errorf("StatusUp should be plain when NO_COLOR is set, got %q", StatusUp)
	}
	if StatusDown != "✗ down" {
		t.Errorf("StatusDown should be plain when NO_COLOR is set, got %q", StatusDown)
	}
}

func TestInit_NoColor_AnyValue(t *testing.T) {
	// The NO_COLOR spec says presence matters, not value.
	t.Cleanup(resetStyles)
	t.Setenv("NO_COLOR", "1")
	Init()

	if _, ok := Green.(lipgloss.NoColor); !ok {
		t.Error("Green should be NoColor when NO_COLOR=1")
	}
}

func TestInit_NoColor_PreservesBold(t *testing.T) {
	t.Cleanup(resetStyles)
	t.Setenv("NO_COLOR", "")
	Init()

	if !ProjectStyle.GetBold() {
		t.Error("ProjectStyle should preserve Bold when NO_COLOR is set")
	}
	if !PortStyle.GetBold() {
		t.Error("PortStyle should preserve Bold when NO_COLOR is set")
	}
}

func TestInit_DefaultColors(t *testing.T) {
	// When NO_COLOR is NOT set and we're not in a real terminal (CI),
	// HasDarkBackground returns true by default. So we just verify Init
	// doesn't panic and styles remain usable.
	t.Cleanup(resetStyles)
	Init()

	// Styles should still render without error.
	_ = ProjectStyle.Render("test")
	_ = PortStyle.Render("12345")
	_ = DimStyle.Render("dim text")
}

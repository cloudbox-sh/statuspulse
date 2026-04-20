// Package styles holds the central Lip Gloss brand kit for the StatusPulse CLI.
//
// Per the Cloudbox design system: Catppuccin Mocha base palette with a single
// Mauve accent (#cba6f7). Every CLI shares these tokens so terminal output
// feels like the rest of the Cloudbox suite.
package styles

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette + Cloudbox accent.
var (
	ColorBase     = lipgloss.Color("#1e1e2e")
	ColorMantle   = lipgloss.Color("#181825")
	ColorSurface0 = lipgloss.Color("#313244")
	ColorSurface1 = lipgloss.Color("#45475a")
	ColorText     = lipgloss.Color("#cdd6f4")
	ColorSubtext1 = lipgloss.Color("#bac2de")
	ColorSubtext0 = lipgloss.Color("#a6adc8")
	ColorOverlay0 = lipgloss.Color("#6c7086")

	ColorAccent  = lipgloss.Color("#cba6f7") // Mauve — the Cloudbox accent
	ColorSuccess = lipgloss.Color("#a6e3a1") // Green
	ColorWarning = lipgloss.Color("#f9e2af") // Yellow
	ColorError   = lipgloss.Color("#f38ba8") // Red
	ColorInfo    = lipgloss.Color("#89b4fa") // Blue
)

var (
	// Accent renders text in the Cloudbox mauve — use for brand marks and
	// interactive emphasis, not for body copy.
	Accent = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	// Highlight is bold text at full contrast. Use sparingly.
	Highlight = lipgloss.NewStyle().Foreground(ColorText).Bold(true)

	// Dim is for secondary text — labels, hints, timestamps.
	Dim = lipgloss.NewStyle().Foreground(ColorSubtext0)

	// Faint is for tertiary detail — comments, placeholders.
	Faint = lipgloss.NewStyle().Foreground(ColorOverlay0)

	// Semantic colours.
	Success = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	Warning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	Error   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Info    = lipgloss.NewStyle().Foreground(ColorInfo)

	// Code renders inline terminal/code snippets.
	Code = lipgloss.NewStyle().Foreground(ColorAccent)

	// Header is used for table column headers.
	Header = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1)

	// Cell is the default table cell style.
	Cell = lipgloss.NewStyle().
		Foreground(ColorText).
		Padding(0, 1)

	// Border is the 1px divider colour used for tables and cards.
	Border = lipgloss.NewStyle().Foreground(ColorSurface0)
)

// StatusGlyph returns a coloured dot for a monitor/incident status string.
// Unknown statuses render as a dim question mark so missing data is visible.
func StatusGlyph(status string) string {
	switch status {
	case "up", "operational", "resolved":
		return Success.Render("●")
	case "down", "major_outage":
		return Error.Render("●")
	case "degraded", "partial_outage", "investigating", "identified":
		return Warning.Render("●")
	case "monitoring":
		return Info.Render("●")
	case "unknown", "":
		return Faint.Render("○")
	default:
		return Dim.Render("●")
	}
}

// Check returns a green ✓.
func Check() string { return Success.Render("✓") }

// Cross returns a red ✗.
func Cross() string { return Error.Render("✗") }

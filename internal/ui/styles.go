package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	secondaryColor = lipgloss.Color("#06B6D4") // cyan
	successColor   = lipgloss.Color("#10B981") // green
	warningColor   = lipgloss.Color("#F59E0B") // amber
	errorColor     = lipgloss.Color("#EF4444") // red
	subtleColor    = lipgloss.Color("#6B7280") // gray
	textColor      = lipgloss.Color("#F9FAFB") // near-white

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Subtle help text
	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	// Selected item in a list
	selectedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// Normal item
	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Tag pill
	tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
		Foreground(warningColor)

	// Status messages
	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	// Box border for sections
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)
)

// padRight pads a string with spaces to reach the desired width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// lineWithRightAlign renders a left portion padded to colWidth, then appends the right portion.
func lineWithRightAlign(left string, leftPlain int, right string, colWidth int) string {
	pad := colWidth - leftPlain
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

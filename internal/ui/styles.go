package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors — vibrant palette with gradients
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	accentColor    = lipgloss.Color("#A855F7") // lighter purple
	secondaryColor = lipgloss.Color("#06B6D4") // cyan
	successColor   = lipgloss.Color("#10B981") // green
	warningColor   = lipgloss.Color("#F59E0B") // amber
	errorColor     = lipgloss.Color("#EF4444") // red
	subtleColor    = lipgloss.Color("#6B7280") // gray
	dimColor       = lipgloss.Color("#4B5563") // darker gray
	textColor      = lipgloss.Color("#F9FAFB") // near-white
	brightWhite    = lipgloss.Color("#FFFFFF")

	// Gradient colors for progress bars and accents
	gradientColors = []lipgloss.Color{
		"#7C3AED", "#8B5CF6", "#A855F7", "#C084FC", "#D8B4FE",
	}
	progressFilled = lipgloss.Color("#A855F7")
	progressEmpty  = lipgloss.Color("#374151")

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Subtle help text
	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	// Dim text
	dimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Selected item in a list
	selectedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// Normal item
	normalStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Tag pill
	tagStyle = lipgloss.NewStyle().
			Foreground(brightWhite).
			Background(primaryColor).
			Padding(0, 1)

	warningStyle = lipgloss.NewStyle().
		Foreground(warningColor)

	secondaryStyle = lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true)

	// Status messages
	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Box border for sections
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	// Menu item styles
	menuItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Bold(true).
				PaddingLeft(1)

	menuDescStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			PaddingLeft(4).
			Italic(true)

	// Divider
	dividerStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Status bar (footer)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)
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

// logo renders the DFC ASCII art header with gradient coloring.
func logo() string {
	lines := []string{
		`  ╺━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╸`,
		`   ██████╗ ███████╗ ██████╗`,
		`   ██╔══██╗██╔════╝██╔════╝`,
		`   ██║  ██║█████╗  ██║     `,
		`   ██║  ██║██╔══╝  ██║     `,
		`   ██████╔╝██║     ╚██████╗`,
		`   ╚═════╝ ╚═╝      ╚═════╝`,
		`   D O T  F I L E  C O M M A N D E R`,
		`  ╺━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╸`,
	}

	var b strings.Builder
	for i, line := range lines {
		idx := i % len(gradientColors)
		style := lipgloss.NewStyle().Foreground(gradientColors[idx])
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

// divider renders a horizontal rule.
func divider(width int) string {
	if width <= 0 {
		width = 40
	}
	return dividerStyle.Render(strings.Repeat("─", width))
}

// sectionHeader renders a styled section title with an icon.
func sectionHeader(icon, title string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render(icon+" "+title) + "\n" +
		dividerStyle.Render(strings.Repeat("─", len(icon)+1+len(title)+4))
}

// statusBar renders a consistent footer with key hints.
func statusBar(hints string) string {
	return "\n" + statusBarStyle.Render("╶─ "+hints+" ─╴")
}

// renderGradientBar renders a progress bar with gradient coloring.
func renderGradientBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	filled := int(percent * float64(width))
	empty := width - filled

	var bar strings.Builder
	for i := 0; i < filled; i++ {
		idx := i * len(gradientColors) / width
		if idx >= len(gradientColors) {
			idx = len(gradientColors) - 1
		}
		style := lipgloss.NewStyle().Foreground(gradientColors[idx])
		bar.WriteString(style.Render("█"))
	}
	emptyStyle := lipgloss.NewStyle().Foreground(progressEmpty)
	bar.WriteString(emptyStyle.Render(strings.Repeat("░", empty)))

	pct := int(percent * 100)
	pctStr := lipgloss.NewStyle().Foreground(accentColor).Render(
		fmt.Sprintf(" %d%%", pct),
	)

	return helpStyle.Render("│") + bar.String() + helpStyle.Render("│") + pctStr
}

// spinner frames for animated indicators
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// menuDescriptions provides subtitle text for each menu item.
var menuDescriptions = []string{
	"Push dotfiles to your git repo",
	"Pull dotfiles from your git repo",
	"Add, remove, or tag entries",
	"View sync status of all entries",
	"Reset all tracking data",
	"Set this machine's identity",
	"Configure repository settings",
}

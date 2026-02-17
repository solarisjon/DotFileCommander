package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(m.menuItems)-1 {
				m.menuCursor++
			}
		case "enter":
			switch m.menuCursor {
			case 0: // Backup
				m.currentView = viewBackup
				return m, m.startBackup()
			case 1: // Restore
				m.currentView = viewRestore
				m.initRestoreView()
				return m, nil
			case 2: // Manage Entries
				m.currentView = viewEntryList
				m.entryCursor = 0
			case 3: // Settings
				m.currentView = viewSetup
				m.setupStep = 0
			}
			return m, nil
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewMainMenu() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📁 DFC — Dot File Commander"))
	b.WriteString("\n\n")

	for i, item := range m.menuItems {
		icon := menuIcons[i]
		if i == m.menuCursor {
			b.WriteString(selectedStyle.Render("▸ " + icon + " " + item))
		} else {
			b.WriteString(normalStyle.Render("  " + icon + " " + item))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Show tracked entries count and repo info
	entryCount := len(m.cfg.Entries)
	if entryCount > 0 {
		b.WriteString(helpStyle.Render(
			strings.Join([]string{
				pluralize(entryCount, "entry", "entries") + " tracked",
				"repo: " + m.cfg.RepoURL,
			}, " • ")))
	} else {
		b.WriteString(helpStyle.Render("No entries tracked yet — add some via Manage Entries"))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • q quit"))

	return boxStyle.Render(b.String())
}

var menuIcons = []string{"⬆", "⬇", "📋", "⚙"}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strings.Replace(strings.Replace("N "+plural, "N", string(rune('0'+n%10)), 1), "N", "", -1)
}

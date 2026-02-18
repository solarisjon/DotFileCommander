package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
				if m.needsProfile() {
					m.profileInput.SetValue("")
					m.profileInput.Focus()
					m.profileReturn = viewBackup
					m.currentView = viewProfileEdit
					m.errMsg = ""
					return m, m.profileInput.Focus()
				}
				m.currentView = viewBackup
				return m, m.startBackup()
			case 1: // Restore
				if m.needsProfile() {
					m.profileInput.SetValue("")
					m.profileInput.Focus()
					m.profileReturn = viewRestore
					m.currentView = viewProfileEdit
					m.errMsg = ""
					return m, m.profileInput.Focus()
				}
				m.currentView = viewRestore
				m.initRestoreView()
				return m, nil
			case 2: // Manage Entries
				m.currentView = viewEntryList
				m.entryCursor = 0
			case 3: // Remote Status
				m.currentView = viewRemote
				return m, m.initRemoteView()
			case 4: // Reset
				m.currentView = viewReset
				m.initResetView()
				return m, nil
			case 5: // Device Profile
				m.profileInput.SetValue(m.cfg.DeviceProfile)
				m.profileInput.Focus()
				m.profileReturn = viewMainMenu
				m.currentView = viewProfileEdit
				m.errMsg = ""
				return m, m.profileInput.Focus()
			case 6: // Settings
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

	b.WriteString(logo())
	b.WriteString("\n")

	for i, item := range m.menuItems {
		icon := menuIcons[i]
		if i == m.menuCursor {
			b.WriteString(menuSelectedStyle.Render("▸ " + icon + "  " + item))
			b.WriteString("\n")
			if i < len(menuDescriptions) {
				b.WriteString(menuDescStyle.Render(menuDescriptions[i]))
			}
		} else {
			b.WriteString(menuItemStyle.Render("  " + icon + "  " + dimStyle.Render(item)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(divider(38))
	b.WriteString("\n")

	// Show tracked entries count and repo info
	entryCount := len(m.cfg.Entries)
	if entryCount > 0 {
		b.WriteString(dimStyle.Render("  📊 "))
		b.WriteString(helpStyle.Render(pluralize(entryCount, "entry", "entries") + " tracked"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  🔗 "))
		b.WriteString(helpStyle.Render(m.cfg.RepoURL))
		b.WriteString("\n")
		if m.cfg.DeviceProfile != "" {
			b.WriteString(dimStyle.Render("  👤 "))
			b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render(m.cfg.DeviceProfile))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(helpStyle.Render("  No entries tracked yet — start with Manage Entries"))
		b.WriteString("\n")
	}

	b.WriteString(statusBar("↑/↓ navigate • enter select • q quit"))

	return boxStyle.Render(b.String())
}

var menuIcons = []string{"⬆", "⬇", "📋", "🌐", "🔄", "👤", "⚙"}

// needsProfile returns true if there are profile-specific entries but no device profile set.
func (m Model) needsProfile() bool {
	if m.cfg.DeviceProfile != "" {
		return false
	}
	for _, e := range m.cfg.Entries {
		if e.ProfileSpecific {
			return true
		}
	}
	return false
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

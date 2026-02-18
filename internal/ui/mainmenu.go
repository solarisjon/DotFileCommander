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
		info := []string{
			pluralize(entryCount, "entry", "entries") + " tracked",
			"repo: " + m.cfg.RepoURL,
		}
		if m.cfg.DeviceProfile != "" {
			info = append(info, "profile: "+m.cfg.DeviceProfile)
		}
		b.WriteString(helpStyle.Render(strings.Join(info, " • ")))
	} else {
		b.WriteString(helpStyle.Render("No entries tracked yet — add some via Manage Entries"))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • q quit"))

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
	return strings.Replace(strings.Replace("N "+plural, "N", string(rune('0'+n%10)), 1), "N", "", -1)
}

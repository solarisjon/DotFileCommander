package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateProfileEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.currentView = m.profileReturn
			return m, nil
		case "enter":
			profile := strings.ToLower(strings.TrimSpace(m.profileInput.Value()))
			if profile == "" {
				m.errMsg = "Profile name cannot be empty"
				return m, nil
			}
			m.cfg.DeviceProfile = profile
			_ = m.cfg.Save()
			m.errMsg = ""
			m.currentView = m.profileReturn
			// If returning to backup or restore, start the action
			switch m.profileReturn {
			case viewBackup:
				return m, m.startBackup()
			case viewRestore:
				m.initRestoreView()
				return m, nil
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.profileInput, cmd = m.profileInput.Update(msg)
	return m, cmd
}

func (m Model) viewProfileEdit() string {
	var b strings.Builder

	b.WriteString(sectionHeader("👤", "Device Profile"))
	b.WriteString("\n\n")
	b.WriteString("Set a name for this device to support per-machine configurations.\n")
	b.WriteString("Profile-specific entries will be stored separately for each profile.\n\n")

	if m.cfg.DeviceProfile != "" {
		b.WriteString("Current profile: " + selectedStyle.Render(m.cfg.DeviceProfile))
		b.WriteString("\n\n")
	}

	b.WriteString("Profile name:\n\n")
	b.WriteString(m.profileInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Examples: work, home, laptop, server"))

	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	b.WriteString(statusBar("enter save • esc cancel"))

	return boxStyle.Render(b.String())
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
)

func (m Model) updateAddEntry(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.addStep > 0 {
				m.addStep--
				return m, nil
			}
			m.currentView = viewEntryList
			return m, nil

		case "y":
			if m.addStep == 2 {
				m.addProfileSpecific = true
				msg = tea.KeyMsg{Type: tea.KeyEnter}
				return m.updateAddEntry(msg)
			}
		case "n":
			if m.addStep == 2 {
				m.addProfileSpecific = false
				msg = tea.KeyMsg{Type: tea.KeyEnter}
				return m.updateAddEntry(msg)
			}

		case "enter":
			switch m.addStep {
			case 0: // Path entered
				path := strings.TrimSpace(m.addInput.Value())
				if path == "" {
					m.errMsg = "Path cannot be empty"
					return m, nil
				}
				m.addIsDir = entry.IsDir(path)
				m.addNameInput.SetValue(entry.FriendlyName(path))
				m.addStep = 1
				m.addNameInput.Focus()
				m.errMsg = ""
				return m, m.addNameInput.Focus()

			case 1: // Name entered — ask profile-specific
				m.addProfileSpecific = false
				m.addStep = 2
				return m, nil

			case 2: // Profile-specific answered — save
				path := strings.TrimSpace(m.addInput.Value())
				name := strings.TrimSpace(m.addNameInput.Value())

				e := config.Entry{
					Path:            path,
					Name:            name,
					IsDir:           m.addIsDir,
					ProfileSpecific: m.addProfileSpecific,
				}

				if err := m.cfg.AddEntry(e); err != nil {
					m.errMsg = fmt.Sprintf("Error: %v", err)
					return m, nil
				}

				m.currentView = viewEntryList
				m.errMsg = ""
				return m, nil
			}
		}
	}

	// Forward to active input
	var cmd tea.Cmd
	switch m.addStep {
	case 0:
		m.addInput, cmd = m.addInput.Update(msg)
	case 1:
		m.addNameInput, cmd = m.addNameInput.Update(msg)
	}
	return m, cmd
}

func (m Model) viewAddEntry() string {
	var b strings.Builder

	b.WriteString(sectionHeader("➕", "Add Entry"))
	b.WriteString("\n\n")

	steps := []string{"Path", "Friendly Name", "Profile-Specific"}
	for i, step := range steps {
		prefix := "  "
		if i == m.addStep {
			prefix = "▸ "
		}
		if i < m.addStep {
			prefix = "✓ "
		}
		b.WriteString(helpStyle.Render(prefix + step))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	switch m.addStep {
	case 0:
		b.WriteString("Enter file or directory path:\n\n")
		b.WriteString(m.addInput.View())
	case 1:
		b.WriteString(fmt.Sprintf("Path: %s\n\n", helpStyle.Render(m.addInput.Value())))
		b.WriteString("Enter a friendly name:\n\n")
		b.WriteString(m.addNameInput.View())
	case 2:
		b.WriteString(fmt.Sprintf("Path: %s\n", helpStyle.Render(m.addInput.Value())))
		b.WriteString(fmt.Sprintf("Name: %s\n\n", helpStyle.Render(m.addNameInput.Value())))
		b.WriteString("Store a separate copy per device profile? (y/n)\n\n")
		b.WriteString(helpStyle.Render("Profile-specific entries are backed up per device."))
	}

	b.WriteString("\n\n")

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		b.WriteString("\n\n")
	}

	if m.addStep == 2 {
		b.WriteString(statusBar("y yes • n no • esc back"))
	} else {
		b.WriteString(statusBar("enter next • esc back"))
	}

	return boxStyle.Render(b.String())
}

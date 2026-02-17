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

		case "enter":
			switch m.addStep {
			case 0: // Path entered
				path := strings.TrimSpace(m.addInput.Value())
				if path == "" {
					m.errMsg = "Path cannot be empty"
					return m, nil
				}
				m.addIsDir = entry.IsDir(path)
				// Pre-fill friendly name
				m.addNameInput.SetValue(entry.FriendlyName(path))
				m.addStep = 1
				m.addNameInput.Focus()
				m.errMsg = ""
				return m, m.addNameInput.Focus()

			case 1: // Name entered
				m.addStep = 2
				m.addTagInput.Focus()
				return m, m.addTagInput.Focus()

			case 2: // Tags entered — save
				path := strings.TrimSpace(m.addInput.Value())
				name := strings.TrimSpace(m.addNameInput.Value())
				tagsStr := strings.TrimSpace(m.addTagInput.Value())

				var tags []string
				if tagsStr != "" {
					for _, t := range strings.Split(tagsStr, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							tags = append(tags, t)
						}
					}
				}

				e := config.Entry{
					Path:  path,
					Name:  name,
					IsDir: m.addIsDir,
					Tags:  tags,
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
	case 2:
		m.addTagInput, cmd = m.addTagInput.Update(msg)
	}
	return m, cmd
}

func (m Model) viewAddEntry() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("➕ Add Entry"))
	b.WriteString("\n\n")

	steps := []string{"Path", "Friendly Name", "Tags"}
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
		b.WriteString("Enter tags (comma-separated):\n\n")
		b.WriteString(m.addTagInput.View())
	}

	b.WriteString("\n\n")

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("enter next • esc back"))

	return boxStyle.Render(b.String())
}

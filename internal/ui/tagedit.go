package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateTagEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.currentView = viewEntryList
			return m, nil
		case "enter":
			tagsStr := strings.TrimSpace(m.tagInput.Value())
			var tags []string
			if tagsStr != "" {
				for _, t := range strings.Split(tagsStr, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}

			e := m.cfg.Entries[m.tagEditIdx]
			e.Tags = tags
			_ = m.cfg.UpdateEntry(m.tagEditIdx, e)
			m.currentView = viewEntryList
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.tagInput, cmd = m.tagInput.Update(msg)
	return m, cmd
}

func (m Model) viewTagEdit() string {
	var b strings.Builder

	e := m.cfg.Entries[m.tagEditIdx]

	b.WriteString(titleStyle.Render("🏷  Edit Tags"))
	b.WriteString("\n\n")
	b.WriteString("Entry: " + selectedStyle.Render(e.Name) + " " + helpStyle.Render(e.Path))
	b.WriteString("\n\n")
	b.WriteString("Tags (comma-separated):\n\n")
	b.WriteString(m.tagInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("enter save • esc cancel"))

	return boxStyle.Render(b.String())
}

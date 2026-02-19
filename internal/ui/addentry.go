package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
)

// buildAddForm creates a huh form for adding a new entry.
// Phase 1 (addStep==0): path only. Phase 2 (addStep==1): name + profile toggle.
func (m *Model) buildAddForm() tea.Cmd {
	if m.addStep == 0 {
		m.addForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Key("path").
					Title("File or directory path").
					Description("e.g. ~/.config/kitty or ~/.bashrc").
					Placeholder("~/.config/kitty").
					Value(&m.addPath),
			),
		).WithWidth(m.contentWidth()).
			WithShowHelp(false).
			WithShowErrors(true).
			WithTheme(dfcHuhTheme())
		return m.addForm.Init()
	}

	// Phase 2: name + profile-specific
	m.addForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("name").
				Title("Friendly name").
				Description("Display name for this entry").
				Value(&m.addName),
			huh.NewConfirm().
				Key("profile").
				Title("Profile-specific?").
				Description("Store a separate copy per device profile").
				Affirmative("Yes").
				Negative("No").
				Value(&m.addProfileSpecific),
		),
	).WithWidth(m.contentWidth()).
		WithShowHelp(false).
		WithShowErrors(true).
		WithTheme(dfcHuhTheme())
	return m.addForm.Init()
}

func (m Model) updateAddEntry(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.addForm == nil {
		return m, nil
	}

	// Intercept esc before forwarding to huh
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		if m.addStep > 0 {
			m.addStep = 0
			m.errMsg = ""
			cmd := m.buildAddForm()
			return m, cmd
		}
		m.currentView = viewEntryList
		m.buildEntryList()
		return m, nil
	}

	// Forward to huh form
	form, cmd := m.addForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.addForm = f
	}

	if m.addForm.State == huh.StateCompleted {
		if m.addStep == 0 {
			// Phase 1 done — validate path and move to phase 2
			path := strings.TrimSpace(m.addPath)
			if path == "" {
				m.errMsg = "Path cannot be empty"
				cmd := m.buildAddForm()
				return m, cmd
			}
			m.addIsDir = entry.IsDir(path)
			m.addName = entry.FriendlyName(path)
			m.addProfileSpecific = false
			m.addStep = 1
			initCmd := m.buildAddForm()
			return m, initCmd
		}

		// Phase 2 done — save entry
		path := strings.TrimSpace(m.addPath)
		name := strings.TrimSpace(m.addName)

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
		m.buildEntryList()
		m.errMsg = ""
		return m, nil
	}

	return m, cmd
}

func (m Model) viewAddEntry() string {
	var b strings.Builder

	b.WriteString(sectionHeader("➕", "Add Entry"))
	b.WriteString("\n\n")

	if m.addStep > 0 {
		b.WriteString(successStyle.Render("✓ Path: "))
		b.WriteString(helpStyle.Render(m.addPath))
		b.WriteString("\n\n")
	}

	if m.addForm != nil {
		b.WriteString(m.addForm.View())
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	b.WriteString("\n")
	b.WriteString(statusBar("esc back"))

	return m.box().Render(b.String())
}

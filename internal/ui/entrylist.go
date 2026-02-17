package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/manifest"
)

func (m Model) updateEntryList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.entryCursor > 0 {
				m.entryCursor--
			}
		case "down", "j":
			if m.entryCursor < len(m.cfg.Entries) {
				m.entryCursor++
			}
		case "a":
			// Add new entry
			m.currentView = viewAddEntry
			m.addStep = 0
			m.addInput.SetValue("")
			m.addNameInput.SetValue("")
			m.addTagInput.SetValue("")
			m.addInput.Focus()
			return m, m.addInput.Focus()
		case "d", "delete", "backspace":
			// Delete selected entry
			if len(m.cfg.Entries) > 0 && m.entryCursor < len(m.cfg.Entries) {
				_ = m.cfg.RemoveEntry(m.entryCursor)
				if m.entryCursor >= len(m.cfg.Entries) && m.entryCursor > 0 {
					m.entryCursor--
				}
			}
		case "t":
			// Edit tags
			if len(m.cfg.Entries) > 0 && m.entryCursor < len(m.cfg.Entries) {
				m.tagEditIdx = m.entryCursor
				e := m.cfg.Entries[m.entryCursor]
				m.tagInput.SetValue(strings.Join(e.Tags, ", "))
				m.tagInput.Focus()
				m.currentView = viewTagEdit
				return m, m.tagInput.Focus()
			}
		case "b":
			// Browse ~/.config
			m.browserStep = 0
			m.browserCursor = 0
			m.browserTagInput.SetValue("")
			m.browserTagInput.Focus()
			m.currentView = viewConfigBrowser
			return m, m.browserTagInput.Focus()
		case "esc", "q":
			m.currentView = viewMainMenu
		}
	}
	return m, nil
}

func (m Model) viewEntryList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📋 Tracked Entries"))
	b.WriteString("\n\n")

	if len(m.cfg.Entries) == 0 {
		b.WriteString(helpStyle.Render("No entries yet. Press 'a' to add one."))
		b.WriteString("\n\n")
	} else {
		// Load manifest for version display
		mf, _ := manifest.Load(m.cfg.RepoPath)

		// Compute column widths
		maxName := 0
		maxPath := 0
		for _, e := range m.cfg.Entries {
			name := e.Name
			if name == "" {
				name = entry.FriendlyName(e.Path)
			}
			if len(name) > maxName {
				maxName = len(name)
			}
			if len(e.Path) > maxPath {
				maxPath = len(e.Path)
			}
		}

		// Two-pass: build lines with plain-text widths, then right-align versions
		type entryLine struct {
			left      string
			leftWidth int // plain-text character count
			ver       string
		}
		lines := make([]entryLine, len(m.cfg.Entries))
		maxLeft := 0

		for i, e := range m.cfg.Entries {
			name := e.Name
			if name == "" {
				name = entry.FriendlyName(e.Path)
			}

			icon := "📄"
			if e.IsDir {
				icon = "📁"
			}

			tags := ""
			tagsPlain := 0
			if len(e.Tags) > 0 {
				tagPills := make([]string, len(e.Tags))
				for j, t := range e.Tags {
					tagPills[j] = tagStyle.Render(t)
					tagsPlain += len(t) + 2 // pill padding
				}
				tags = strings.Join(tagPills, " ")
				tagsPlain += len(e.Tags) - 1 // spaces between pills
			}

			nameCol := padRight(name, maxName+2)
			pathCol := padRight(e.Path, maxPath+2)

			left := fmt.Sprintf("%s %s %s %s", icon, nameCol, helpStyle.Render(pathCol), tags)
			// plain width: icon(2) + space + name + space + path + space + tags
			leftWidth := 2 + 1 + (maxName + 2) + 1 + (maxPath + 2) + 1 + tagsPlain

			verInfo := ""
			if mf != nil {
				repoVer := mf.GetVersion(e.Path)
				if repoVer > 0 {
					if e.LocalVersion < repoVer {
						verInfo = warningStyle.Render(fmt.Sprintf("⬆ v%d→v%d", e.LocalVersion, repoVer))
					} else {
						verInfo = successStyle.Render(fmt.Sprintf("v%d", repoVer))
					}
				}
			}

			lines[i] = entryLine{left: left, leftWidth: leftWidth, ver: verInfo}
			if leftWidth > maxLeft {
				maxLeft = leftWidth
			}
		}

		for i, el := range lines {
			var line string
			if el.ver != "" {
				line = lineWithRightAlign(el.left, el.leftWidth, el.ver, maxLeft+2)
			} else {
				line = el.left
			}

			if i == m.entryCursor {
				b.WriteString(selectedStyle.Render("▸ ") + line)
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("a add • b browse ~/.config • d delete • t tags • esc back"))

	return boxStyle.Render(b.String())
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/manifest"
	"github.com/solarisjon/dfc/internal/storage"
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
			m.addProfileSpecific = false
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
		case "b":
			// Browse ~/.config
			m.browserCursor = 0
			m.currentView = viewConfigBrowser
			m.initBrowserDirs()
			return m, nil
		case "p":
			// Toggle profile-specific
			if len(m.cfg.Entries) > 0 && m.entryCursor < len(m.cfg.Entries) {
				m.cfg.Entries[m.entryCursor].ProfileSpecific = !m.cfg.Entries[m.entryCursor].ProfileSpecific
				_ = m.cfg.Save()
			}
		case "esc", "q":
			m.currentView = viewMainMenu
		}
	}
	return m, nil
}

func (m Model) viewEntryList() string {
	var b strings.Builder

	b.WriteString(sectionHeader("📋", "Tracked Entries"))
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
		// Cap columns to fit terminal width
		cw := m.contentWidth()
		avail := cw - 12 // icon(4) + spaces + ver hint(~8)
		if avail < 20 {
			avail = 20
		}
		nameLimit := avail * 2 / 5
		pathLimit := avail - nameLimit
		if maxName > nameLimit {
			maxName = nameLimit
		}
		if maxPath > pathLimit {
			maxPath = pathLimit
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
			if e.ProfileSpecific {
				icon += "👤"
			}

			nameCol := padRight(name, maxName+2)
			pathCol := padRight(e.Path, maxPath+2)

			left := fmt.Sprintf("%s %s %s", icon, nameCol, helpStyle.Render(pathCol))
			// plain width: icon(2+2 if profile) + space + name + space + path
			iconWidth := 2
			if e.ProfileSpecific {
				iconWidth = 4
			}
			leftWidth := iconWidth + 1 + (maxName + 2) + 1 + (maxPath + 2)

			verInfo := ""
			if mf != nil {
				mkey := storage.ManifestKey(e, m.cfg.DeviceProfile)
				repoVer := mf.GetVersion(mkey)
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

	b.WriteString(statusBar("a add • b browse • d delete • p profile • esc back"))

	return m.box().Render(b.String())
}

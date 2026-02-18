package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
)

type browserItem struct {
	name     string // directory name under ~/.config
	friendly string // human-readable name
	selected bool
	tracked  bool // already in config entries
}

const (
	browserStepTags   = 0 // ask for tags first
	browserStepSelect = 1 // then pick directories
)

func (m *Model) initBrowserDirs() {
	dirs, err := entry.ListConfigDirs()
	if err != nil {
		m.errMsg = fmt.Sprintf("Cannot read ~/.config: %v", err)
		return
	}

	// Build a set of already-tracked paths for fast lookup
	tracked := make(map[string]bool)
	for _, e := range m.cfg.Entries {
		tracked[e.Path] = true
	}

	m.browserDirs = make([]browserItem, 0, len(dirs))
	for _, d := range dirs {
		path := filepath.Join("~/.config", d)
		friendly := entry.FriendlyName(path)
		m.browserDirs = append(m.browserDirs, browserItem{
			name:     d,
			friendly: friendly,
			selected: false,
			tracked:  tracked[path],
		})
	}
}

func (m Model) updateConfigBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.browserStep == browserStepTags {
			return m.updateBrowserTags(msg)
		}
		return m.updateBrowserSelect(msg)
	}

	// Pass through to text input in tag step
	if m.browserStep == browserStepTags {
		var cmd tea.Cmd
		m.browserTagInput, cmd = m.browserTagInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateBrowserTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Move to selection step
		m.browserStep = browserStepSelect
		m.browserCursor = 0
		m.initBrowserDirs()
		return m, nil
	case "esc":
		m.currentView = viewEntryList
		return m, nil
	}

	var cmd tea.Cmd
	m.browserTagInput, cmd = m.browserTagInput.Update(msg)
	return m, cmd
}

func (m Model) updateBrowserSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.browserCursor > 0 {
			m.browserCursor--
		}
	case "down", "j":
		if m.browserCursor < len(m.browserDirs)-1 {
			m.browserCursor++
		}
	case " ":
		// Toggle current item
		if m.browserCursor < len(m.browserDirs) && !m.browserDirs[m.browserCursor].tracked {
			m.browserDirs[m.browserCursor].selected = !m.browserDirs[m.browserCursor].selected
		}
	case "a":
		// Select all (untracked)
		for i := range m.browserDirs {
			if !m.browserDirs[i].tracked {
				m.browserDirs[i].selected = true
			}
		}
	case "n":
		// Select none
		for i := range m.browserDirs {
			m.browserDirs[i].selected = false
		}
	case "enter":
		return m.commitBrowserSelection()
	case "esc", "q":
		m.currentView = viewEntryList
		return m, nil
	}
	return m, nil
}

func (m Model) commitBrowserSelection() (tea.Model, tea.Cmd) {
	// Parse tags from input
	tagStr := m.browserTagInput.Value()
	var tags []string
	if strings.TrimSpace(tagStr) != "" {
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	added := 0
	for _, item := range m.browserDirs {
		if !item.selected || item.tracked {
			continue
		}
		path := filepath.Join("~/.config", item.name)
		e := config.Entry{
			Path:  path,
			Name:  item.friendly,
			Tags:  tags,
			IsDir: true,
		}
		if err := m.cfg.AddEntry(e); err != nil {
			m.errMsg = fmt.Sprintf("Failed to add %s: %v", item.name, err)
			break
		}
		added++
	}

	if added > 0 && m.errMsg == "" {
		m.statusMsg = fmt.Sprintf("Added %d %s", added, pluralize2(added))
	}

	m.currentView = viewEntryList
	m.entryCursor = 0
	return m, nil
}

func pluralize2(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}

func (m Model) viewConfigBrowser() string {
	var b strings.Builder

	b.WriteString(sectionHeader("📂", "Browse ~/.config"))
	b.WriteString("\n\n")

	if m.browserStep == browserStepTags {
		b.WriteString(normalStyle.Render("Tags to apply to all selected entries:"))
		b.WriteString("\n\n")
		b.WriteString(m.browserTagInput.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Comma-separated, e.g. home, work, laptop"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("Leave blank for no tags"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter continue • esc cancel"))
		return boxStyle.Render(b.String())
	}

	if len(m.browserDirs) == 0 {
		b.WriteString(helpStyle.Render("No directories found in ~/.config"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return boxStyle.Render(b.String())
	}

	// Show tag summary
	tagStr := m.browserTagInput.Value()
	if strings.TrimSpace(tagStr) != "" {
		b.WriteString(helpStyle.Render("Tags: "))
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				b.WriteString(tagStyle.Render(t) + " ")
			}
		}
		b.WriteString("\n\n")
	}

	// Count selected
	selCount := 0
	for _, item := range m.browserDirs {
		if item.selected {
			selCount++
		}
	}

	// Calculate visible window for scrolling
	maxVisible := 15
	start := 0
	if len(m.browserDirs) > maxVisible {
		// Center cursor in window
		start = m.browserCursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		if start+maxVisible > len(m.browserDirs) {
			start = len(m.browserDirs) - maxVisible
		}
	}
	end := start + maxVisible
	if end > len(m.browserDirs) {
		end = len(m.browserDirs)
	}

	if start > 0 {
		b.WriteString(helpStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Compute column widths for visible items
	maxFriendly := 0
	maxDirName := 0
	for _, item := range m.browserDirs {
		if len(item.friendly) > maxFriendly {
			maxFriendly = len(item.friendly)
		}
		if item.friendly != item.name && len(item.name) > maxDirName {
			maxDirName = len(item.name)
		}
	}

	for i := start; i < end; i++ {
		item := m.browserDirs[i]
		checkbox := "[ ]"
		nameStyle := normalStyle

		if item.tracked {
			checkbox = successStyle.Render("[✓]")
			nameStyle = helpStyle // dim already-tracked items
		} else if item.selected {
			checkbox = selectedStyle.Render("[✓]")
		}

		nameCol := padRight(item.friendly, maxFriendly+2)
		dirCol := ""
		if item.friendly != item.name {
			dirCol = helpStyle.Render(padRight("("+item.name+")", maxDirName+4))
		} else if maxDirName > 0 {
			dirCol = padRight("", maxDirName+4)
		}

		status := ""
		if item.tracked {
			status = helpStyle.Render("already tracked")
		}

		line := fmt.Sprintf("%s 📁 %s %s %s", checkbox, nameStyle.Render(nameCol), dirCol, status)

		if i == m.browserCursor {
			b.WriteString(selectedStyle.Render("▸ ") + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if end < len(m.browserDirs) {
		b.WriteString(helpStyle.Render("  ↓ more"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d/%d selected", selCount, len(m.browserDirs))))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("space toggle • a all • n none • enter add • esc cancel"))

	return boxStyle.Render(b.String())
}

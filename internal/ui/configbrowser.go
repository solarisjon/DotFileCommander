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
	path     string // full path, e.g. "~/.config/kitty" or "~/.zshrc"
	name     string // display name (base dir/file name)
	friendly string // human-readable name
	selected bool
	tracked  bool // already in config entries
	isDir    bool
	isHeader bool // non-selectable section separator
}

func (m *Model) initBrowserDirs() {
	tracked := make(map[string]bool)
	for _, e := range m.cfg.Entries {
		tracked[e.Path] = true
	}

	m.browserDirs = nil

	// Section 1: ~/.config subdirectories
	configDirs, err := entry.ListConfigDirs()
	if err != nil {
		m.errMsg = fmt.Sprintf("Cannot read ~/.config: %v", err)
	} else if len(configDirs) > 0 {
		m.browserDirs = append(m.browserDirs, browserItem{isHeader: true, friendly: "~/.config"})
		for _, d := range configDirs {
			path := filepath.Join("~/.config", d)
			m.browserDirs = append(m.browserDirs, browserItem{
				path:    path,
				name:    d,
				friendly: entry.FriendlyName(path),
				tracked: tracked[path],
				isDir:   true,
			})
		}
	}

	// Section 2: dotfiles/dotdirs directly in ~
	homeDots, err2 := entry.ListHomeDotfiles()
	if err2 != nil {
		m.errMsg = fmt.Sprintf("Cannot read ~: %v", err2)
	} else if len(homeDots) > 0 {
		m.browserDirs = append(m.browserDirs, browserItem{isHeader: true, friendly: "~  (dotfiles)"})
		for _, d := range homeDots {
			path := filepath.Join("~", d)
			m.browserDirs = append(m.browserDirs, browserItem{
				path:    path,
				name:    d,
				friendly: entry.FriendlyName(path),
				tracked: tracked[path],
				isDir:   entry.IsDir(path),
			})
		}
	}
}

func (m Model) updateConfigBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			for m.browserCursor > 0 {
				m.browserCursor--
				if !m.browserDirs[m.browserCursor].isHeader {
					break
				}
			}
		case "down", "j":
			for m.browserCursor < len(m.browserDirs)-1 {
				m.browserCursor++
				if !m.browserDirs[m.browserCursor].isHeader {
					break
				}
			}
		case " ":
			if m.browserCursor < len(m.browserDirs) && !m.browserDirs[m.browserCursor].tracked && !m.browserDirs[m.browserCursor].isHeader {
				m.browserDirs[m.browserCursor].selected = !m.browserDirs[m.browserCursor].selected
			}
		case "a":
			for i := range m.browserDirs {
				if !m.browserDirs[i].tracked && !m.browserDirs[i].isHeader {
					m.browserDirs[i].selected = true
				}
			}
		case "n":
			for i := range m.browserDirs {
				if !m.browserDirs[i].isHeader {
					m.browserDirs[i].selected = false
				}
			}
		case "enter":
			return m.commitBrowserSelection()
		case "esc", "q":
			m.currentView = viewEntryList
			m.buildEntryList()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) commitBrowserSelection() (tea.Model, tea.Cmd) {
	added := 0
	for _, item := range m.browserDirs {
		if item.isHeader || !item.selected || item.tracked {
			continue
		}
		e := config.Entry{
			Path:  item.path,
			Name:  item.friendly,
			IsDir: item.isDir,
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
	m.buildEntryList()
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

	b.WriteString(sectionHeader("📂", "Browse Dotfiles"))
	b.WriteString("\n\n")

	if len(m.browserDirs) == 0 {
		b.WriteString(helpStyle.Render("No dotfiles found"))
		b.WriteString("\n\n")
		b.WriteString(statusBar("esc back"))
		return m.box().Render(b.String())
	}

	// Count selected (non-header items only)
	selCount := 0
	selectableCount := 0
	for _, item := range m.browserDirs {
		if item.isHeader {
			continue
		}
		selectableCount++
		if item.selected {
			selCount++
		}
	}

	// Calculate visible window for scrolling
	maxVisible := m.listHeight(10)
	start := 0
	if len(m.browserDirs) > maxVisible {
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

	// Compute column widths
	maxFriendly := 0
	maxName := 0
	for _, item := range m.browserDirs {
		if item.isHeader {
			continue
		}
		if len(item.friendly) > maxFriendly {
			maxFriendly = len(item.friendly)
		}
		if item.friendly != item.name && len(item.name) > maxName {
			maxName = len(item.name)
		}
	}
	cw := m.contentWidth()
	avail := cw - 20
	if avail < 16 {
		avail = 16
	}
	friendlyLimit := avail * 3 / 5
	nameLimit := avail - friendlyLimit
	if maxFriendly > friendlyLimit {
		maxFriendly = friendlyLimit
	}
	if maxName > nameLimit {
		maxName = nameLimit
	}

	for i := start; i < end; i++ {
		item := m.browserDirs[i]

		if item.isHeader {
			b.WriteString("\n")
			b.WriteString(dimStyle.Render("  ── " + item.friendly + " ──"))
			b.WriteString("\n")
			continue
		}

		icon := "📄"
		if item.isDir {
			icon = "📁"
		}
		checkbox := "[ ]"
		nameStyle := normalStyle

		if item.tracked {
			checkbox = successStyle.Render("[✓]")
			nameStyle = helpStyle
		} else if item.selected {
			checkbox = selectedStyle.Render("[✓]")
		}

		nameCol := padRight(item.friendly, maxFriendly+2)
		dirCol := ""
		if item.friendly != item.name {
			dirCol = helpStyle.Render(padRight("("+item.name+")", maxName+4))
		} else if maxName > 0 {
			dirCol = padRight("", maxName+4)
		}

		status := ""
		if item.tracked {
			status = helpStyle.Render("already tracked")
		}

		line := fmt.Sprintf("%s %s %s %s %s", checkbox, icon, nameStyle.Render(nameCol), dirCol, status)

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
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d/%d selected", selCount, selectableCount)))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("space toggle • a all • n none • enter add • esc cancel"))

	return m.box().Render(b.String())
}

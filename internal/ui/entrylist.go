package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/manifest"
	"github.com/solarisjon/dfc/internal/storage"
)

// entryItem implements list.DefaultItem for the entry list.
type entryItem struct {
	index           int
	name            string
	path            string
	isDir           bool
	profileSpecific bool
	verInfo         string // pre-rendered version info
}

func (i entryItem) Title() string       { return i.name }
func (i entryItem) Description() string { return i.path }
func (i entryItem) FilterValue() string { return i.name + " " + i.path }

// entryDelegate renders entries with icons and version info.
type entryDelegate struct {
	styles list.DefaultItemStyles
}

func newEntryDelegate() entryDelegate {
	s := list.NewDefaultItemStyles()
	s.NormalTitle = s.NormalTitle.Foreground(textColor).Bold(false)
	s.NormalDesc = s.NormalDesc.Foreground(subtleColor)
	s.SelectedTitle = s.SelectedTitle.Foreground(secondaryColor).Bold(true).
		BorderLeftForeground(accentColor)
	s.SelectedDesc = s.SelectedDesc.Foreground(accentColor).
		BorderLeftForeground(accentColor)
	return entryDelegate{styles: s}
}

func (d entryDelegate) Height() int                             { return 2 }
func (d entryDelegate) Spacing() int                            { return 0 }
func (d entryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d entryDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(entryItem)
	if !ok {
		return
	}

	icon := "📄"
	if i.isDir {
		icon = "📁"
	}
	if i.profileSpecific {
		icon += " 👤"
	}

	title := fmt.Sprintf("%s  %s", icon, i.name)
	desc := i.path
	if i.verInfo != "" {
		desc += "  " + i.verInfo
	}

	var titleStyle, descStyle lipgloss.Style
	if index == m.Index() {
		titleStyle = d.styles.SelectedTitle
		descStyle = d.styles.SelectedDesc
	} else {
		titleStyle = d.styles.NormalTitle
		descStyle = d.styles.NormalDesc
	}

	fmt.Fprintf(w, "%s\n%s", titleStyle.Render(title), descStyle.Render(desc))
}

func (m *Model) buildEntryList() {
	mf, _ := manifest.Load(m.cfg.RepoPath)

	items := make([]list.Item, len(m.cfg.Entries))
	for idx, e := range m.cfg.Entries {
		name := e.Name
		if name == "" {
			name = entry.FriendlyName(e.Path)
		}

		verInfo := ""
		if mf != nil {
			mkey := storage.ManifestKey(e, m.cfg.DeviceProfile)
			repoVer := mf.GetVersion(mkey)
			if repoVer > 0 {
				if e.LocalVersion < repoVer {
					verInfo = fmt.Sprintf("⬆ v%d→v%d", e.LocalVersion, repoVer)
				} else {
					verInfo = fmt.Sprintf("v%d ✓", repoVer)
				}
			}
		}

		items[idx] = entryItem{
			index:           idx,
			name:            name,
			path:            e.Path,
			isDir:           e.IsDir,
			profileSpecific: e.ProfileSpecific,
			verInfo:         verInfo,
		}
	}

	cw := m.contentWidth()
	delegate := newEntryDelegate()

	l := list.New(items, delegate, cw, 14)
	l.Title = "📋 Tracked Entries"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).MarginLeft(0)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false) // we show our own
	l.DisableQuitKeybindings()
	l.SetStatusBarItemName("entry", "entries")

	m.entryList = &l
}

func (m Model) updateEntryList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when filtering
		if m.entryList != nil && m.entryList.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "a":
			m.currentView = viewAddEntry
			m.addStep = 0
			m.addPath = ""
			m.addName = ""
			m.addProfileSpecific = false
			m.errMsg = ""
			cmd := m.buildAddForm()
			return m, cmd
		case "d", "delete", "backspace":
			if m.entryList != nil {
				if sel, ok := m.entryList.SelectedItem().(entryItem); ok {
					_ = m.cfg.RemoveEntry(sel.index)
					m.buildEntryList()
				}
			}
			return m, nil
		case "b":
			m.browserCursor = 0
			m.currentView = viewConfigBrowser
			m.initBrowserDirs()
			return m, nil
		case "p":
			if m.entryList != nil {
				if sel, ok := m.entryList.SelectedItem().(entryItem); ok {
					m.cfg.Entries[sel.index].ProfileSpecific = !m.cfg.Entries[sel.index].ProfileSpecific
					_ = m.cfg.Save()
					m.buildEntryList()
				}
			}
			return m, nil
		case "esc":
			if m.entryList != nil && m.entryList.IsFiltered() {
				m.entryList.ResetFilter()
				return m, nil
			}
			m.currentView = viewMainMenu
			return m, nil
		case "q":
			m.currentView = viewMainMenu
			return m, nil
		}
	}

	if m.entryList != nil {
		l, cmd := m.entryList.Update(msg)
		m.entryList = &l
		return m, cmd
	}
	return m, nil
}

func (m Model) viewEntryList() string {
	var b strings.Builder

	if m.entryList == nil || len(m.cfg.Entries) == 0 {
		b.WriteString(sectionHeader("📋", "Tracked Entries"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("No entries yet. Press 'a' to add one, or 'b' to browse ~/.config."))
		b.WriteString("\n")
		b.WriteString(statusBar("a add • b browse • esc back"))
		return m.box().Render(b.String())
	}

	b.WriteString(m.entryList.View())
	b.WriteString(statusBar("a add • b browse • d delete • p profile • / filter • esc back"))

	return m.box().Render(b.String())
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/restore"
	"github.com/solarisjon/dfc/internal/storage"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

type bootstrapSyncDoneMsg struct{ err error }
type bootstrapProgressMsg restore.Progress

type bootstrapItem struct {
	entry    storage.RepoEntry
	selected bool
}

const (
	bootstrapStepSyncing = 0
	bootstrapStepSelect  = 1
	bootstrapStepRunning = 2
)

func (m *Model) initBootstrapView() tea.Cmd {
	m.bootstrapStep = bootstrapStepSyncing
	m.bootstrapCursor = 0
	m.bootstrapEntries = nil
	m.bootstrapCh = nil
	m.progressItems = nil
	m.progressDone = false
	m.errMsg = ""

	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return bootstrapSyncDoneMsg{err: err}
	}
}

func (m *Model) updateBootstrapView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case bootstrapSyncDoneMsg:
		if msg.err != nil {
			m.errMsg = "Sync failed: " + msg.err.Error()
			m.bootstrapStep = bootstrapStepSelect
			return m, nil
		}
		repoEntries, err := storage.ListRepoEntries(m.cfg.RepoPath, m.cfg.DeviceProfile, m.cfg.Entries)
		if err != nil {
			m.errMsg = "Failed to read repo: " + err.Error()
			m.bootstrapStep = bootstrapStepSelect
			return m, nil
		}
		m.bootstrapEntries = make([]bootstrapItem, len(repoEntries))
		for i, re := range repoEntries {
			m.bootstrapEntries[i] = bootstrapItem{entry: re, selected: true}
		}
		m.bootstrapStep = bootstrapStepSelect
		return m, nil

	case bootstrapProgressMsg:
		p := restore.Progress(msg)
		if p.Index < len(m.progressItems) {
			pi := &m.progressItems[p.Index]
			if p.BytesTotal > 0 {
				pi.percent = float64(p.BytesCopied) / float64(p.BytesTotal)
			} else {
				pi.percent = 1.0
			}
			if p.Done {
				pi.done = true
				pi.err = p.Err
				pi.skipped = p.Skipped
				pi.skipReasons = p.SkipReasons
			}
		}
		if !p.Done {
			return m, m.waitBootstrapProgress()
		}
		for _, pi := range m.progressItems {
			if !pi.done {
				return m, m.waitBootstrapProgress()
			}
		}
		m.progressDone = true
		return m, nil

	case tea.KeyMsg:
		switch m.bootstrapStep {
		case bootstrapStepSelect:
			switch msg.String() {
			case "up", "k":
				if m.bootstrapCursor > 0 {
					m.bootstrapCursor--
				}
			case "down", "j":
				if m.bootstrapCursor < len(m.bootstrapEntries)-1 {
					m.bootstrapCursor++
				}
			case " ":
				if m.bootstrapCursor < len(m.bootstrapEntries) {
					m.bootstrapEntries[m.bootstrapCursor].selected = !m.bootstrapEntries[m.bootstrapCursor].selected
				}
			case "a":
				for i := range m.bootstrapEntries {
					m.bootstrapEntries[i].selected = true
				}
			case "n":
				for i := range m.bootstrapEntries {
					m.bootstrapEntries[i].selected = false
				}
			case "enter":
				return m, m.startBootstrapRestore()
			case "esc":
				m.currentView = viewMainMenu
			}
		case bootstrapStepRunning:
			if m.progressDone {
				switch msg.String() {
				case "enter", "esc":
					m.currentView = viewMainMenu
				}
			}
		}
	}

	return m, nil
}

func (m *Model) startBootstrapRestore() tea.Cmd {
	var selected []bootstrapItem
	for _, bi := range m.bootstrapEntries {
		if bi.selected {
			selected = append(selected, bi)
		}
	}
	if len(selected) == 0 {
		m.currentView = viewMainMenu
		return nil
	}

	// Add selected entries to local config and save
	for _, bi := range selected {
		m.cfg.Entries = append(m.cfg.Entries, bi.entry.Entry)
	}
	_ = m.cfg.Save()

	// Build progress items
	entries := make([]config.Entry, len(selected))
	m.progressItems = make([]progressItem, len(selected))
	for i, bi := range selected {
		entries[i] = bi.entry.Entry
		name := bi.entry.Entry.Name
		if name == "" {
			name = bi.entry.Entry.Path
		}
		m.progressItems[i] = progressItem{name: name}
	}
	m.progressDone = false
	m.bootstrapStep = bootstrapStepRunning

	ch := restore.Run(entries, m.cfg.RepoPath, m.cfg.DeviceProfile)
	m.bootstrapCh = ch
	return m.waitBootstrapProgress()
}

func (m *Model) waitBootstrapProgress() tea.Cmd {
	ch := m.bootstrapCh
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return bootstrapProgressMsg(p)
	}
}

func (m Model) viewBootstrap() string {
	var b strings.Builder

	switch m.bootstrapStep {
	case bootstrapStepSyncing:
		b.WriteString(titleStyle.Render("📦 Import from Repo"))
		b.WriteString("\n")
		b.WriteString(divider(m.contentWidth() * 2 / 3))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  ⟳ Syncing repo..."))
		b.WriteString("\n\n")
		b.WriteString(statusBar("please wait"))

	case bootstrapStepSelect:
		b.WriteString(titleStyle.Render("📦 Import from Repo — Select Entries"))
		b.WriteString("\n")
		b.WriteString(divider(m.contentWidth() * 2 / 3))
		b.WriteString("\n\n")

		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("  ✗ " + m.errMsg))
			b.WriteString("\n\n")
		}

		if len(m.bootstrapEntries) == 0 && m.errMsg == "" {
			b.WriteString(successStyle.Render("  ✓ All repo entries are already tracked on this machine"))
			b.WriteString("\n\n")
			b.WriteString(statusBar("esc back to menu"))
			break
		}

		selectedCount := 0
		for i, bi := range m.bootstrapEntries {
			icon := "📄"
			if bi.entry.Entry.IsDir {
				icon = "📁"
			}
			profileIcon := ""
			if bi.entry.Entry.ProfileSpecific {
				profileIcon = " 👤"
			}
			check := "[ ]"
			if bi.selected {
				check = "[✓]"
				selectedCount++
			}
			versionStr := fmt.Sprintf("v%d", bi.entry.Version)
			line := fmt.Sprintf("%s %s %s  %s%s", check, icon, bi.entry.Entry.Name, versionStr, profileIcon)

			if i == m.bootstrapCursor {
				b.WriteString(menuSelectedStyle.Render("  ▸ " + line))
			} else {
				b.WriteString("    " + dimStyle.Render(line))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d selected", selectedCount, len(m.bootstrapEntries))))
		b.WriteString("\n")
		b.WriteString(statusBar("space toggle • a all • n none • enter import • esc back"))

	case bootstrapStepRunning:
		b.WriteString(titleStyle.Render("📦 Importing"))
		b.WriteString("\n")
		b.WriteString(divider(m.contentWidth() * 2 / 3))
		b.WriteString("\n\n")

		for _, item := range m.progressItems {
			var status string
			if item.done {
				if item.err != nil {
					status = errorStyle.Render("✗")
				} else if item.skipped > 0 {
					status = warningStyle.Render("⚠")
				} else {
					status = successStyle.Render("✓")
				}
			} else {
				status = lipgloss.NewStyle().Foreground(accentColor).Render("⟳")
			}
			cw := m.contentWidth()
			nameW := cw*2/5 - 4
			barW := cw * 2 / 5
			name := padRight(item.name, nameW)
			bar := renderGradientBar(item.percent, barW)
			b.WriteString(fmt.Sprintf(" %s  %s %s", status, name, bar))
			if item.err != nil {
				b.WriteString(" " + errorStyle.Render(item.err.Error()))
			} else if item.skipped > 0 {
				b.WriteString(" " + warningStyle.Render(fmt.Sprintf("%d skipped", item.skipped)))
				for _, reason := range item.skipReasons {
					b.WriteString("\n      " + helpStyle.Render("  · "+reason))
				}
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		if m.progressDone {
			allOK := true
			for _, pi := range m.progressItems {
				if pi.err != nil {
					allOK = false
				}
			}
			if allOK {
				b.WriteString(successStyle.Render("  ✓ Import complete! Entries added to config."))
			} else {
				b.WriteString(errorStyle.Render("  ✗ Some entries failed"))
			}
			b.WriteString("\n")
			b.WriteString(statusBar("enter/esc back to menu"))
		} else {
			b.WriteString(statusBar("importing..."))
		}
	}

	return m.box().Render(b.String())
}

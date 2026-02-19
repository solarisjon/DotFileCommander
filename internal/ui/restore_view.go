package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/manifest"
	"github.com/solarisjon/dfc/internal/restore"
	"github.com/solarisjon/dfc/internal/storage"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

type restoreProgressMsg restore.Progress

// restoreSyncDoneMsg signals repo sync completed (before restore runs).
type restoreSyncDoneMsg struct{ err error }

// restorePreSyncDoneMsg signals the initial repo sync before showing entries.
type restorePreSyncDoneMsg struct{ err error }

const (
	restoreStepSyncing = 0 // syncing repo before showing entries
	restoreStepEntries = 1 // select entries to restore
	restoreStepRunning = 2 // progress view
)

type restoreEntryItem struct {
	entry    config.Entry
	idx      int // original index in cfg.Entries
	selected bool
	conflict restore.ConflictState
	inRepo   bool // whether entry exists in the repo
}

func (m *Model) initRestoreView() tea.Cmd {
	m.restoreCursor = 0
	m.restoreStep = restoreStepSyncing
	m.progressDone = false
	m.errMsg = ""
	m.statusMsg = ""
	m.progressItems = nil
	m.restoreCh = nil
	m.restoreEntries = nil

	// Sync repo first, then build entries after sync completes
	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return restorePreSyncDoneMsg{err: err}
	}
}

func (m *Model) buildRestoreEntries() {
	filtered := m.cfg.Entries

	// Check conflicts
	var conflicts []restore.ConflictResult
	if m.restoreManifest != nil {
		conflicts = restore.CheckConflicts(filtered, m.restoreManifest, m.cfg.DeviceProfile)
	}

	repoPath := expandHome(m.cfg.RepoPath)

	m.restoreEntries = make([]restoreEntryItem, len(filtered))
	for i, e := range filtered {
		// Check if entry exists in repo
		relPath := storage.RepoDir(e, m.cfg.DeviceProfile)
		srcPath := filepath.Join(repoPath, relPath)
		_, statErr := os.Stat(srcPath)

		item := restoreEntryItem{
			entry:    e,
			idx:      i,
			selected: statErr == nil, // only pre-select if exists in repo
			inRepo:   statErr == nil,
		}
		if conflicts != nil {
			item.conflict = conflicts[i].State
		}
		m.restoreEntries[i] = item
	}
}

func (m Model) startRestore() tea.Cmd {
	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return restoreSyncDoneMsg{err: err}
	}
}

func (m *Model) runRestore() tea.Cmd {
	// Collect selected entries
	var entries []config.Entry
	for _, item := range m.restoreEntries {
		if item.selected {
			entries = append(entries, item.entry)
		}
	}

	m.progressItems = make([]progressItem, len(entries))
	for i, e := range entries {
		name := e.Name
		if name == "" {
			name = e.Path
		}
		m.progressItems[i] = progressItem{name: name}
	}
	m.progressDone = false

	ch := restore.Run(entries, m.cfg.RepoPath, m.cfg.DeviceProfile)
	m.restoreCh = ch

	return waitForRestoreProgress(ch)
}

func waitForRestoreProgress(ch <-chan restore.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return restoreProgressMsg{Done: true}
		}
		return restoreProgressMsg(p)
	}
}

func (m Model) handleRestoreSyncDone(msg restoreSyncDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = fmt.Sprintf("Repo sync failed: %v", msg.err)
		m.progressDone = true
		return m, nil
	}
	return m, m.runRestore()
}

func (m Model) handleRestoreProgress(msg restoreProgressMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.progressItems) {
		item := &m.progressItems[msg.Index]
		item.done = msg.Done
		item.err = msg.Err
		item.skipped = msg.Skipped
		item.skipReasons = msg.SkipReasons
		if msg.BytesTotal > 0 {
			item.percent = float64(msg.BytesCopied) / float64(msg.BytesTotal)
		} else if msg.Done {
			item.percent = 1.0
		}
	}

	allDone := true
	for _, item := range m.progressItems {
		if !item.done {
			allDone = false
			break
		}
	}

	if allDone {
		m.progressDone = true

		// Update local versions and hashes from manifest for successfully restored entries
		mf, err := manifest.Load(m.cfg.RepoPath)
		if err == nil {
			var restored []config.Entry
			for _, item := range m.restoreEntries {
				if item.selected {
					restored = append(restored, item.entry)
				}
			}
			for i, item := range m.progressItems {
				if item.done && item.err == nil && i < len(restored) {
					for j := range m.cfg.Entries {
						if m.cfg.Entries[j].Path == restored[i].Path {
							mkey := storage.ManifestKey(m.cfg.Entries[j], m.cfg.DeviceProfile)
							m.cfg.Entries[j].LocalVersion = mf.GetVersion(mkey)
							m.cfg.Entries[j].LastHash = mf.Entries[mkey].ContentHash
							break
						}
					}
				}
			}
			_ = m.cfg.Save()
		}

		m.statusMsg = "Restore complete!"
		return m, nil
	}

	if m.restoreCh != nil {
		return m, waitForRestoreProgress(m.restoreCh)
	}
	return m, nil
}

func (m Model) updateRestoreView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.restoreStep {
		case restoreStepSyncing:
			if msg.String() == "esc" || msg.String() == "q" {
				m.currentView = viewMainMenu
				return m, nil
			}
		case restoreStepEntries:
			return m.updateRestoreEntries(msg)
		case restoreStepRunning:
			return m.updateRestoreRunning(msg)
		}
	case restorePreSyncDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Repo sync failed: %v", msg.err)
			// Still build entries from local state so user can see what's available
		}
		// Now that repo is synced, load manifest and build entries
		m.restoreManifest, _ = manifest.Load(m.cfg.RepoPath)
		m.buildRestoreEntries()
		m.restoreStep = restoreStepEntries
		return m, nil
	case restoreSyncDoneMsg:
		return m.handleRestoreSyncDone(msg)
	}
	return m, nil
}

func (m Model) updateRestoreEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.restoreCursor > 0 {
			m.restoreCursor--
		}
	case "down", "j":
		if m.restoreCursor < len(m.restoreEntries)-1 {
			m.restoreCursor++
		}
	case " ":
		if m.restoreCursor < len(m.restoreEntries) {
			m.restoreEntries[m.restoreCursor].selected = !m.restoreEntries[m.restoreCursor].selected
		}
	case "a":
		for i := range m.restoreEntries {
			m.restoreEntries[i].selected = true
		}
	case "n":
		for i := range m.restoreEntries {
			m.restoreEntries[i].selected = false
		}
	case "enter":
		count := 0
		hasConflicts := false
		for _, item := range m.restoreEntries {
			if item.selected {
				count++
				if item.conflict == restore.StateModifiedLocal ||
					item.conflict == restore.StateConflict ||
					item.conflict == restore.StateNewerInRepo {
					hasConflicts = true
				}
			}
		}
		if count == 0 {
			m.errMsg = "No entries selected"
			return m, nil
		}
		if hasConflicts && !m.restoreConfirmed {
			m.restoreConfirmed = true
			m.errMsg = "⚠ Some local files will be overwritten! Press enter again to confirm, or deselect them."
			return m, nil
		}
		m.errMsg = ""
		m.restoreConfirmed = false
		m.restoreStep = restoreStepRunning
		return m, m.startRestore()
	case "esc", "q":
		m.restoreConfirmed = false
		m.currentView = viewMainMenu
		return m, nil
	}
	m.restoreConfirmed = false
	m.errMsg = ""
	return m, nil
}

func (m Model) updateRestoreRunning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		if m.progressDone {
			m.currentView = viewMainMenu
			m.errMsg = ""
			m.statusMsg = ""
			m.restoreCh = nil
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewRestoreProgress() string {
	switch m.restoreStep {
	case restoreStepSyncing:
		return m.viewRestoreSyncing()
	case restoreStepEntries:
		return m.viewRestoreEntries()
	case restoreStepRunning:
		return m.viewRestoreRunning()
	}
	return ""
}

func (m Model) viewRestoreSyncing() string {
	var b strings.Builder

	b.WriteString(sectionHeader("⬇", "Restore"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render("⟳ "))
	b.WriteString(normalStyle.Render("Syncing repository..."))

	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(statusBar("esc back"))

	return m.box().Render(b.String())
}

func (m Model) viewRestoreEntries() string {
	var b strings.Builder

	b.WriteString(sectionHeader("⬇", "Restore — Select Entries"))
	b.WriteString("\n\n")

	if len(m.restoreEntries) == 0 {
		b.WriteString(helpStyle.Render("No entries to restore."))
		b.WriteString("\n\n")
		b.WriteString(statusBar("esc back"))
		return m.box().Render(b.String())
	}

	selCount := 0
	for _, item := range m.restoreEntries {
		if item.selected {
			selCount++
		}
	}

	// Scrollable list
	maxVisible := m.listHeight(10) // header + selected count + help + chrome
	start := 0
	if len(m.restoreEntries) > maxVisible {
		start = m.restoreCursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		if start+maxVisible > len(m.restoreEntries) {
			start = len(m.restoreEntries) - maxVisible
		}
	}
	end := start + maxVisible
	if end > len(m.restoreEntries) {
		end = len(m.restoreEntries)
	}

	if start > 0 {
		b.WriteString(helpStyle.Render("  ↑ more"))
		b.WriteString("\n")
	}

	// Compute column widths
	maxName := 0
	for _, item := range m.restoreEntries {
		name := item.entry.Name
		if name == "" {
			name = entry.FriendlyName(item.entry.Path)
		}
		if len(name) > maxName {
			maxName = len(name)
		}
	}
	// Cap to fit terminal
	cw := m.contentWidth()
	nameLimit := cw/2 - 10
	if nameLimit < 10 {
		nameLimit = 10
	}
	if maxName > nameLimit {
		maxName = nameLimit
	}

	// Two-pass: build lines, then right-align versions
	type rl struct {
		left      string
		leftWidth int
		ver       string
	}
	rlines := make([]rl, end-start)
	maxLeft := 0

	for idx, i := 0, start; i < end; idx, i = idx+1, i+1 {
		item := m.restoreEntries[i]
		check := "[ ]"
		if item.selected {
			check = selectedStyle.Render("[✓]")
		}

		name := item.entry.Name
		if name == "" {
			name = entry.FriendlyName(item.entry.Path)
		}
		icon := "📄"
		if item.entry.IsDir {
			icon = "📁"
		}
		if item.entry.ProfileSpecific {
			icon += "👤"
		}

		nameCol := padRight(name, maxName+2)

		left := fmt.Sprintf("%s %s %s", check, icon, nameCol)
		iconWidth := 2
		if item.entry.ProfileSpecific {
			iconWidth = 4
		}
		leftWidth := 3 + 1 + iconWidth + 1 + (maxName + 2)

		verInfo := ""
		// Show repo availability
		if !item.inRepo {
			verInfo = helpStyle.Render("not in repo")
		} else if m.restoreManifest != nil {
			mkey := storage.ManifestKey(item.entry, m.cfg.DeviceProfile)
			repoVer := m.restoreManifest.GetVersion(mkey)
			localVer := item.entry.LocalVersion
			if repoVer > 0 {
				if localVer < repoVer {
					verInfo = warningStyle.Render(fmt.Sprintf("⬆ v%d→v%d", localVer, repoVer))
				} else {
					verInfo = successStyle.Render(fmt.Sprintf("v%d ✓", repoVer))
				}
			}
		}

		// Add conflict state indicator
		switch item.conflict {
		case restore.StateNewerInRepo:
			verInfo += " " + warningStyle.Render("⬇ will overwrite local")
		case restore.StateModifiedLocal:
			verInfo += " " + warningStyle.Render("⚠ modified locally")
		case restore.StateConflict:
			verInfo += " " + errorStyle.Render("⚡ conflict — both changed")
		}

		rlines[idx] = rl{left: left, leftWidth: leftWidth, ver: verInfo}
		if leftWidth > maxLeft {
			maxLeft = leftWidth
		}
	}

	for idx, i := 0, start; i < end; idx, i = idx+1, i+1 {
		el := rlines[idx]
		var line string
		if el.ver != "" {
			line = lineWithRightAlign(el.left, el.leftWidth, el.ver, maxLeft+2)
		} else {
			line = el.left
		}

		if i == m.restoreCursor {
			b.WriteString(selectedStyle.Render("▸ ") + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if end < len(m.restoreEntries) {
		b.WriteString(helpStyle.Render("  ↓ more"))
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d/%d selected", selCount, len(m.restoreEntries))))
	b.WriteString("\n\n")
	b.WriteString(statusBar("space toggle • a all • n none • enter restore • esc back"))

	return m.box().Render(b.String())
}

func (m Model) viewRestoreRunning() string {
	var b strings.Builder

	b.WriteString(sectionHeader("⬇", "Restore"))
	b.WriteString("\n\n")

	if len(m.progressItems) == 0 && !m.progressDone {
		b.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render("⟳ "))
		b.WriteString(normalStyle.Render("Syncing repository..."))
		if m.errMsg != "" {
			b.WriteString("\n\n")
			b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		}
	} else {
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
			line := fmt.Sprintf(" %s  %s %s", status, name, bar)
			b.WriteString(line)

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
	}

	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ " + m.statusMsg))
	}
	if m.errMsg != "" && len(m.progressItems) > 0 {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	if m.progressDone {
		b.WriteString(statusBar("enter/esc back to menu"))
	} else {
		b.WriteString(statusBar("restoring..."))
	}

	return m.box().Render(b.String())
}

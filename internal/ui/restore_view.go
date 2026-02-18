package ui

import (
	"fmt"
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

// restoreSyncDoneMsg signals repo sync completed.
type restoreSyncDoneMsg struct{ err error }

const (
	restoreStepTags    = 0 // pick tags to filter by
	restoreStepEntries = 1 // select entries to restore
	restoreStepRunning = 2 // progress view
)

type restoreTagItem struct {
	tag      string
	selected bool
}

type restoreEntryItem struct {
	entry    config.Entry
	idx      int // original index in cfg.Entries
	selected bool
	conflict restore.ConflictState
}

func (m *Model) initRestoreView() {
	m.restoreStep = restoreStepTags
	m.restoreCursor = 0
	m.progressDone = false
	m.errMsg = ""
	m.statusMsg = ""
	m.progressItems = nil
	m.restoreCh = nil

	// Load manifest to check versions
	m.restoreManifest, _ = manifest.Load(m.cfg.RepoPath)

	// Collect unique tags
	tagSet := make(map[string]bool)
	for _, e := range m.cfg.Entries {
		for _, t := range e.Tags {
			tagSet[t] = true
		}
	}
	m.restoreTags = nil
	for t := range tagSet {
		m.restoreTags = append(m.restoreTags, restoreTagItem{tag: t, selected: false})
	}
	m.restoreAllTags = true // default: all entries

	// Pre-populate entry list with all entries
	m.buildRestoreEntries()
}

func (m *Model) buildRestoreEntries() {
	// Get selected tags
	var selectedTags []string
	if !m.restoreAllTags {
		for _, t := range m.restoreTags {
			if t.selected {
				selectedTags = append(selectedTags, t.tag)
			}
		}
	}

	// Filter entries
	var filtered []config.Entry
	if m.restoreAllTags || len(selectedTags) == 0 {
		filtered = m.cfg.Entries
	} else {
		filtered = restore.FilterByTags(m.cfg.Entries, selectedTags)
	}

	// Check conflicts
	var conflicts []restore.ConflictResult
	if m.restoreManifest != nil {
		conflicts = restore.CheckConflicts(filtered, m.restoreManifest, m.cfg.DeviceProfile)
	}

	m.restoreEntries = make([]restoreEntryItem, len(filtered))
	for i, e := range filtered {
		item := restoreEntryItem{
			entry:    e,
			idx:      i,
			selected: true, // default all selected
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
			// Build list of restored entries to match progress items
			var restored []config.Entry
			for _, item := range m.restoreEntries {
				if item.selected {
					restored = append(restored, item.entry)
				}
			}
			for i, item := range m.progressItems {
				if item.done && item.err == nil && i < len(restored) {
					// Find this entry in cfg and update its local version + hash
					for j := range m.cfg.Entries {
						if m.cfg.Entries[j].Path == restored[i].Path {
							mkey := storage.ManifestKey(m.cfg.Entries[j], m.cfg.DeviceProfile)
							m.cfg.Entries[j].LocalVersion = mf.GetVersion(mkey)
							// Hash the restored content so future modifications can be detected
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
		case restoreStepTags:
			return m.updateRestoreTags(msg)
		case restoreStepEntries:
			return m.updateRestoreEntries(msg)
		case restoreStepRunning:
			return m.updateRestoreRunning(msg)
		}
	case restoreSyncDoneMsg:
		return m.handleRestoreSyncDone(msg)
	}
	return m, nil
}

func (m Model) updateRestoreTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.restoreCursor > 0 {
			m.restoreCursor--
		}
	case "down", "j":
		max := len(m.restoreTags) // "All" is index 0 conceptually, tags follow
		if m.restoreCursor < max {
			m.restoreCursor++
		}
	case " ":
		if m.restoreCursor == 0 {
			// Toggle "All"
			m.restoreAllTags = !m.restoreAllTags
			if m.restoreAllTags {
				for i := range m.restoreTags {
					m.restoreTags[i].selected = false
				}
			}
		} else {
			idx := m.restoreCursor - 1
			if idx < len(m.restoreTags) {
				m.restoreTags[idx].selected = !m.restoreTags[idx].selected
				// If any tag is toggled, deselect "All"
				anySelected := false
				for _, t := range m.restoreTags {
					if t.selected {
						anySelected = true
						break
					}
				}
				m.restoreAllTags = !anySelected
			}
		}
		m.buildRestoreEntries()
	case "enter":
		m.restoreStep = restoreStepEntries
		m.restoreCursor = 0
		return m, nil
	case "esc", "q":
		m.currentView = viewMainMenu
		return m, nil
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
		// Count selected
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
		// If there are entries that will change local files, confirm
		if hasConflicts && !m.restoreConfirmed {
			m.restoreConfirmed = true
			m.errMsg = "⚠ Some local files will be overwritten! Press enter again to confirm, or deselect them."
			return m, nil
		}
		m.errMsg = ""
		m.restoreConfirmed = false
		m.restoreStep = restoreStepRunning
		return m, m.startRestore()
	case "esc":
		m.restoreConfirmed = false
		m.restoreStep = restoreStepTags
		m.restoreCursor = 0
		return m, nil
	}
	// Reset confirmation if user changes selection
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
	case restoreStepTags:
		return m.viewRestoreTags()
	case restoreStepEntries:
		return m.viewRestoreEntries()
	case restoreStepRunning:
		return m.viewRestoreRunning()
	}
	return ""
}

func (m Model) viewRestoreTags() string {
	var b strings.Builder

	b.WriteString(sectionHeader("⬇", "Restore — Filter by Tags"))
	b.WriteString("\n\n")

	if len(m.restoreTags) == 0 {
		b.WriteString(helpStyle.Render("No tags found. All entries will be shown."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter continue • esc cancel"))
		return boxStyle.Render(b.String())
	}

	b.WriteString(normalStyle.Render("Select which tags to restore:"))
	b.WriteString("\n\n")

	// "All" option
	allCheck := "( )"
	if m.restoreAllTags {
		allCheck = selectedStyle.Render("(•)")
	}
	allLine := fmt.Sprintf("%s All entries", allCheck)
	if m.restoreCursor == 0 {
		b.WriteString(selectedStyle.Render("▸ " + allLine))
	} else {
		b.WriteString("  " + allLine)
	}
	b.WriteString("\n")

	// Individual tags
	for i, t := range m.restoreTags {
		check := "[ ]"
		if t.selected {
			check = selectedStyle.Render("[✓]")
		}
		line := fmt.Sprintf("%s %s", check, tagStyle.Render(t.tag))

		if i+1 == m.restoreCursor {
			b.WriteString(selectedStyle.Render("▸ ") + line)
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	// Show match count
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(fmt.Sprintf("%d %s will be shown", len(m.restoreEntries), pluralize2(len(m.restoreEntries)))))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("space toggle • enter continue • esc cancel"))

	return boxStyle.Render(b.String())
}

func (m Model) viewRestoreEntries() string {
	var b strings.Builder

	b.WriteString(sectionHeader("⬇", "Restore — Select Entries"))
	b.WriteString("\n\n")

	if len(m.restoreEntries) == 0 {
		b.WriteString(helpStyle.Render("No entries match the selected tags."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return boxStyle.Render(b.String())
	}

	selCount := 0
	for _, item := range m.restoreEntries {
		if item.selected {
			selCount++
		}
	}

	// Scrollable list
	maxVisible := 15
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

		tags := ""
		tagsPlain := 0
		if len(item.entry.Tags) > 0 {
			pills := make([]string, len(item.entry.Tags))
			for j, t := range item.entry.Tags {
				pills[j] = tagStyle.Render(t)
				tagsPlain += len(t) + 2
			}
			tags = strings.Join(pills, " ")
			tagsPlain += len(item.entry.Tags) - 1
		}

		nameCol := padRight(name, maxName+2)

		left := fmt.Sprintf("%s %s %s %s", check, icon, nameCol, tags)
		// plain: [✓](3) + space + icon(2) + space + name + space + tags
		leftWidth := 3 + 1 + 2 + 1 + (maxName + 2) + 1 + tagsPlain

		verInfo := ""
		if m.restoreManifest != nil {
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
	b.WriteString(helpStyle.Render("space toggle • a all • n none • enter restore • esc back"))

	return boxStyle.Render(b.String())
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
				} else {
					status = successStyle.Render("✓")
				}
			} else {
				status = lipgloss.NewStyle().Foreground(accentColor).Render("⟳")
			}

			name := padRight(item.name, 20)
			bar := renderGradientBar(item.percent, 20)
			line := fmt.Sprintf(" %s  %s %s", status, name, bar)
			b.WriteString(line)

			if item.err != nil {
				b.WriteString(" " + errorStyle.Render(item.err.Error()))
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

	return boxStyle.Render(b.String())
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/backup"
	"github.com/solarisjon/dfc/internal/manifest"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

type backupProgressMsg backup.Progress

// repoSyncDoneMsg signals that EnsureRepo completed (with optional error).
type repoSyncDoneMsg struct{ err error }

func (m Model) startBackup() tea.Cmd {
	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return repoSyncDoneMsg{err: err}
	}
}

// checkBackupConflicts detects entries where the repo was updated by another
// device since our last backup/restore (remote hash differs from our LastHash).
func (m *Model) checkBackupConflicts() []string {
	mf, err := manifest.Load(m.cfg.RepoPath)
	if err != nil {
		return nil
	}
	var conflicts []string
	for _, e := range m.cfg.Entries {
		mv := mf.GetEntry(e.Path)
		if mv.Version == 0 {
			continue // never backed up, no conflict possible
		}
		// If we have a last-known hash and the repo hash differs, another
		// device changed this entry since we last synced.
		if e.LastHash != "" && mv.ContentHash != "" && mv.ContentHash != e.LastHash {
			conflicts = append(conflicts, e.Path)
		}
		// If repo version is ahead of our local version, someone else backed up.
		if mv.Version > e.LocalVersion && e.LocalVersion > 0 {
			// Avoid duplicates
			alreadyAdded := false
			for _, c := range conflicts {
				if c == e.Path {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				conflicts = append(conflicts, e.Path)
			}
		}
	}
	return conflicts
}

func (m *Model) runBackup() tea.Cmd {
	m.progressItems = make([]progressItem, len(m.cfg.Entries))
	for i, e := range m.cfg.Entries {
		name := e.Name
		if name == "" {
			name = e.Path
		}
		m.progressItems[i] = progressItem{name: name}
	}
	m.progressDone = false

	ch := backup.Run(m.cfg.Entries, m.cfg.RepoPath)
	m.backupCh = ch

	return waitForBackupProgress(ch)
}

func waitForBackupProgress(ch <-chan backup.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return backupProgressMsg{Done: true}
		}
		return backupProgressMsg(p)
	}
}

func (m Model) handleRepoSyncDone(msg repoSyncDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = fmt.Sprintf("Repo sync failed: %v", msg.err)
		m.progressDone = true
		return m, nil
	}
	// Check if repo was modified by another device
	conflicts := m.checkBackupConflicts()
	if len(conflicts) > 0 && !m.backupConfirmed {
		m.backupConflicts = conflicts
		return m, nil // show conflict warning, wait for user input
	}
	return m, m.runBackup()
}

func (m Model) handleBackupProgress(msg backupProgressMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.progressItems) {
		item := &m.progressItems[msg.Index]
		item.done = msg.Done
		item.err = msg.Err
		item.contentHash = msg.ContentHash
		if msg.BytesTotal > 0 {
			item.percent = float64(msg.BytesCopied) / float64(msg.BytesTotal)
		} else if msg.Done {
			item.percent = 1.0
		}
	}

	// Check if all done
	allDone := true
	for _, item := range m.progressItems {
		if !item.done {
			allDone = false
			break
		}
	}

	if allDone {
		m.progressDone = true

		// Bump manifest versions for successfully backed-up entries
		mf, err := manifest.Load(m.cfg.RepoPath)
		if err != nil {
			mf = &manifest.Manifest{Entries: make(map[string]manifest.EntryVersion)}
		}
		changed := 0
		for i, item := range m.progressItems {
			if item.done && item.err == nil && i < len(m.cfg.Entries) {
				e := &m.cfg.Entries[i]
				bumped := mf.BumpVersion(e.Path, item.contentHash)
				e.LocalVersion = mf.GetVersion(e.Path)
				e.LastHash = item.contentHash
				if bumped {
					changed++
				}
			}
		}
		_ = mf.Save(m.cfg.RepoPath)
		_ = m.cfg.Save()

		// Commit and push (only if something actually changed)
		if changed > 0 {
			if err := gsync.CommitAndPush(m.cfg.RepoPath, "dfc: backup dotfiles"); err != nil {
				m.errMsg = fmt.Sprintf("Push failed: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Backup complete! %d %s updated.", changed, pluralize2(changed))
			}
		} else {
			m.statusMsg = "Backup complete — all entries already up to date."
		}
		return m, nil
	}

	// Keep reading from the progress channel
	if m.backupCh != nil {
		return m, waitForBackupProgress(m.backupCh)
	}
	return m, nil
}

func (m Model) updateBackupView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			// Confirm backup despite conflicts
			if len(m.backupConflicts) > 0 && !m.backupConfirmed {
				m.backupConfirmed = true
				m.backupConflicts = nil
				return m, m.runBackup()
			}
		case "esc", "q":
			if len(m.backupConflicts) > 0 && !m.backupConfirmed {
				// Cancel backup due to conflicts
				m.currentView = viewMainMenu
				m.backupConflicts = nil
				m.backupConfirmed = false
				m.errMsg = ""
				return m, nil
			}
			if m.progressDone {
				m.currentView = viewMainMenu
				m.errMsg = ""
				m.statusMsg = ""
				m.backupCh = nil
				m.backupConflicts = nil
				m.backupConfirmed = false
				return m, nil
			}
		case "enter":
			if m.progressDone {
				m.currentView = viewMainMenu
				m.errMsg = ""
				m.statusMsg = ""
				m.backupCh = nil
				m.backupConflicts = nil
				m.backupConfirmed = false
				return m, nil
			}
		}
	case repoSyncDoneMsg:
		return m.handleRepoSyncDone(msg)
	}

	return m, nil
}

func (m Model) viewBackupProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("⬆ Backup"))
	b.WriteString("\n\n")

	// Show backup conflict warning if detected
	if len(m.backupConflicts) > 0 && !m.backupConfirmed {
		b.WriteString(errorStyle.Render("⚠ Remote repo was updated by another device!"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("The following entries have newer versions in the repo:"))
		b.WriteString("\n\n")
		for _, path := range m.backupConflicts {
			b.WriteString(warningStyle.Render("  • " + path))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("Backing up will overwrite the remote versions."))
		b.WriteString("\n\n")
		b.WriteString(warningStyle.Render("Press y to continue backup, or esc to cancel"))
		return boxStyle.Render(b.String())
	}

	if len(m.progressItems) == 0 && !m.progressDone {
		b.WriteString("Syncing repository...")
		if m.errMsg != "" {
			b.WriteString("\n\n")
			b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		}
	} else {
		for _, item := range m.progressItems {
			status := "  "
			if item.done {
				if item.err != nil {
					status = errorStyle.Render("✗")
				} else {
					status = successStyle.Render("✓")
				}
			} else {
				status = "⋯"
			}

			bar := renderProgressBar(item.percent, 20)
			line := fmt.Sprintf("%s %s %s", status, item.name, bar)
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

	b.WriteString("\n\n")
	if m.progressDone {
		b.WriteString(helpStyle.Render("enter/esc back to menu"))
	} else {
		b.WriteString(helpStyle.Render("backing up..."))
	}

	return boxStyle.Render(b.String())
}

func renderProgressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	filled := int(percent * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return helpStyle.Render("[") + bar + helpStyle.Render("]")
}

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/manifest"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

const (
	resetStepMenu    = 0 // choose reset type
	resetStepConfirm = 1 // confirm the action
	resetStepWorking = 2 // running
	resetStepDone    = 3
)

const (
	resetTypeLocal  = 0 // local only
	resetTypeRemote = 1 // nuke remote repo
)

type resetNukeDoneMsg struct{ err error }

func (m *Model) initResetView() {
	m.resetStep = resetStepMenu
	m.resetConfirmed = false
	m.resetType = resetTypeLocal
	m.errMsg = ""
	m.statusMsg = ""
}

func (m Model) updateResetView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case resetNukeDoneMsg:
		m.resetStep = resetStepDone
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Remote wipe failed: %v", msg.err)
		} else {
			// Also reset local config entries after successful remote wipe
			m.cfg.Entries = nil
			_ = m.cfg.Save()
			m.statusMsg = "Remote repo wiped and reset complete!"
		}
		return m, nil
	case tea.KeyMsg:
		switch m.resetStep {
		case resetStepMenu:
			switch msg.String() {
			case "up", "k":
				if m.resetType > 0 {
					m.resetType--
				}
			case "down", "j":
				if m.resetType < 1 {
					m.resetType++
				}
			case "enter":
				m.resetStep = resetStepConfirm
				m.resetConfirmed = false
				return m, nil
			case "esc", "q":
				m.currentView = viewMainMenu
				return m, nil
			}
		case resetStepConfirm:
			switch msg.String() {
			case "y", "Y":
				m.resetConfirmed = true
				if m.resetType == resetTypeLocal {
					if err := m.performReset(); err != nil {
						m.errMsg = fmt.Sprintf("Reset failed: %v", err)
					} else {
						m.statusMsg = "Local reset complete!"
					}
					m.resetStep = resetStepDone
					return m, nil
				}
				// Remote wipe — run async
				m.resetStep = resetStepWorking
				return m, m.performRemoteWipe()
			case "esc", "q", "n", "N":
				m.resetStep = resetStepMenu
				return m, nil
			}
		case resetStepDone:
			switch msg.String() {
			case "enter", "esc", "q":
				m.currentView = viewMainMenu
				m.errMsg = ""
				m.statusMsg = ""
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *Model) performReset() error {
	// 1. Remove the local repo clone
	repoPath := expandHome(m.cfg.RepoPath)
	if _, err := os.Stat(repoPath); err == nil {
		if err := os.RemoveAll(repoPath); err != nil {
			return fmt.Errorf("removing repo at %s: %w", repoPath, err)
		}
	}

	// 2. Clear all entries and reset config (keep repo URL and path)
	m.cfg.Entries = nil
	if err := m.cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// 3. The manifest lives in the repo, so it's already gone.
	_ = (&manifest.Manifest{Entries: make(map[string]manifest.EntryVersion)}).Save(m.cfg.RepoPath)

	return nil
}

func (m *Model) performRemoteWipe() tea.Cmd {
	repoURL := m.cfg.RepoURL
	repoPath := m.cfg.RepoPath
	return func() tea.Msg {
		// Ensure we have a local clone to work with
		if err := gsync.EnsureRepo(repoURL, repoPath); err != nil {
			return resetNukeDoneMsg{err: fmt.Errorf("syncing repo: %w", err)}
		}
		// Nuke the remote
		if err := gsync.NukeRepo(repoPath); err != nil {
			return resetNukeDoneMsg{err: err}
		}
		return resetNukeDoneMsg{}
	}
}

func (m Model) viewResetView() string {
	var b strings.Builder

	b.WriteString(sectionHeader("🔄", "Reset System"))
	b.WriteString("\n\n")

	switch m.resetStep {
	case resetStepMenu:
		b.WriteString(normalStyle.Render("Choose reset type:"))
		b.WriteString("\n\n")

		options := []struct {
			title string
			desc  string
			icon  string
		}{
			{"Local Reset", "Remove local clone and tracked entries. Remote repo is unchanged.", "🧹"},
			{"Full Remote Wipe", "Destroy all remote repo content, history, and data. Nuclear option!", "💣"},
		}

		for i, opt := range options {
			cursor := "  "
			if i == m.resetType {
				cursor = selectedStyle.Render("▸ ")
			}
			title := opt.icon + " " + opt.title
			if i == m.resetType {
				title = selectedStyle.Render(title)
			}
			b.WriteString(cursor + title)
			b.WriteString("\n")
			b.WriteString("    " + helpStyle.Render(opt.desc))
			b.WriteString("\n\n")
		}

		b.WriteString(statusBar("enter select • esc back"))

	case resetStepConfirm:
		if m.resetType == resetTypeLocal {
			b.WriteString(warningStyle.Render("⚠ Local Reset — this will:"))
			b.WriteString("\n\n")
			b.WriteString(normalStyle.Render("  • Remove the local git repo clone"))
			b.WriteString("\n")
			b.WriteString(normalStyle.Render("  • Clear all tracked entries from config"))
			b.WriteString("\n")
			b.WriteString(normalStyle.Render("  • Reset all version and hash tracking"))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Your dotfiles and the remote repo will NOT be affected."))
		} else {
			b.WriteString(errorStyle.Render("💣 FULL REMOTE WIPE — this will:"))
			b.WriteString("\n\n")
			b.WriteString(errorStyle.Render("  • DELETE all files from the remote repo"))
			b.WriteString("\n")
			b.WriteString(errorStyle.Render("  • DESTROY all git history and versions"))
			b.WriteString("\n")
			b.WriteString(errorStyle.Render("  • FORCE PUSH a clean slate to the remote"))
			b.WriteString("\n\n")
			b.WriteString(normalStyle.Render("  • Clear all tracked entries from local config"))
			b.WriteString("\n")
			b.WriteString(normalStyle.Render("  • Remove local repo clone"))
			b.WriteString("\n\n")
			b.WriteString(warningStyle.Render("This is IRREVERSIBLE. All other devices will need to re-backup."))
			b.WriteString("\n")
			b.WriteString(helpStyle.Render("Your original dotfiles will NOT be deleted."))
		}
		b.WriteString("\n\n")
		b.WriteString(warningStyle.Render("Press y to confirm, or esc/n to go back"))

	case resetStepWorking:
		b.WriteString(normalStyle.Render("💣 Wiping remote repository..."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("This may take a moment."))

	case resetStepDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		} else {
			b.WriteString(successStyle.Render("✓ " + m.statusMsg))
			b.WriteString("\n\n")
			if m.resetType == resetTypeRemote {
				b.WriteString(helpStyle.Render("Remote repo is clean. Re-add entries and backup to start fresh."))
			} else {
				b.WriteString(helpStyle.Render("You can re-add entries and backup again."))
			}
		}
		b.WriteString("\n\n")
		b.WriteString(statusBar("enter/esc back to menu"))
	}

	return boxStyle.Render(b.String())
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/manifest"
)

const (
	resetStepConfirm = 0
	resetStepDone    = 1
)

func (m *Model) initResetView() {
	m.resetStep = resetStepConfirm
	m.resetConfirmed = false
	m.errMsg = ""
	m.statusMsg = ""
}

func (m Model) updateResetView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.resetStep {
		case resetStepConfirm:
			switch msg.String() {
			case "y", "Y":
				m.resetConfirmed = true
				// Perform the reset
				if err := m.performReset(); err != nil {
					m.errMsg = fmt.Sprintf("Reset failed: %v", err)
				} else {
					m.statusMsg = "System reset complete!"
				}
				m.resetStep = resetStepDone
				return m, nil
			case "esc", "q", "n", "N":
				m.currentView = viewMainMenu
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
	// If somehow a stale manifest exists, clear it.
	_ = (&manifest.Manifest{Entries: make(map[string]manifest.EntryVersion)}).Save(m.cfg.RepoPath)

	return nil
}

func (m Model) viewResetView() string {
	var b strings.Builder

	b.WriteString(sectionHeader("🔄", "Reset System"))
	b.WriteString("\n\n")

	switch m.resetStep {
	case resetStepConfirm:
		b.WriteString(errorStyle.Render("⚠ WARNING: This will:"))
		b.WriteString("\n\n")
		b.WriteString(normalStyle.Render("  • Remove the local git repo clone"))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("  • Clear all tracked entries from config"))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("  • Reset all version and hash tracking"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Your original dotfiles will NOT be deleted."))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("The remote git repo will NOT be affected."))
		b.WriteString("\n\n")
		b.WriteString(warningStyle.Render("Press y to confirm reset, or esc/n to cancel"))

	case resetStepDone:
		if m.errMsg != "" {
			b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		} else {
			b.WriteString(successStyle.Render("✓ " + m.statusMsg))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("You can re-add entries and backup again."))
		}
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter/esc back to menu"))
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

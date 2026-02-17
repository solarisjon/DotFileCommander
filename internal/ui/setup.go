package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

// setupStep constants for clarity
const (
	setupStepGhCheck   = 0 // checking gh status
	setupStepChoose    = 1 // choose: existing URL or create new
	setupStepInput     = 2 // enter URL or repo name
	setupStepWorking   = 3 // creating repo / cloning
)

type ghCheckDoneMsg struct{ status gsync.GhStatus }
type ghAuthDoneMsg struct{ err error }
type repoCreateDoneMsg struct {
	url string
	err error
}
type repoCloneDoneMsg struct{ err error }

func (m Model) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case ghCheckDoneMsg:
		m.ghStatus = msg.status
		if msg.status == gsync.GhReady {
			// Configure git to use gh for auth
			_ = gsync.SetupGitCredentialHelper()
			m.setupStep = setupStepChoose
		}
		return m, nil

	case ghAuthDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Authentication failed: %v", msg.err)
			return m, nil
		}
		_ = gsync.SetupGitCredentialHelper()
		m.ghStatus = gsync.GhReady
		m.setupStep = setupStepChoose
		m.errMsg = ""
		return m, nil

	case repoCreateDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to create repo: %v", msg.err)
			m.setupStep = setupStepInput
			return m, nil
		}
		m.cfg.RepoURL = msg.url
		if err := m.cfg.Save(); err != nil {
			m.errMsg = fmt.Sprintf("Error saving config: %v", err)
			m.setupStep = setupStepInput
			return m, nil
		}
		m.statusMsg = "Repository created!"
		m.setupStep = setupStepWorking
		return m, m.cloneRepo()

	case repoCloneDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to clone repo: %v", msg.err)
			m.setupStep = setupStepInput
			return m, nil
		}
		m.errMsg = ""
		m.statusMsg = ""
		m.currentView = viewMainMenu
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.setupStep > setupStepChoose {
				m.setupStep = setupStepChoose
				m.errMsg = ""
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			return m.handleSetupEnter()

		case "up", "k":
			if m.setupStep == setupStepGhCheck && m.ghStatus != gsync.GhReady {
				// no-op
			} else if m.setupStep == setupStepChoose && m.setupMethod > 0 {
				m.setupMethod--
			}
		case "down", "j":
			if m.setupStep == setupStepChoose && m.setupMethod < 1 {
				m.setupMethod++
			}
		}
	}

	if m.setupStep == setupStepInput {
		var cmd tea.Cmd
		m.setupInput, cmd = m.setupInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleSetupEnter() (tea.Model, tea.Cmd) {
	switch m.setupStep {
	case setupStepGhCheck:
		if m.ghStatus == gsync.GhNotInstalled {
			m.errMsg = "Please install the GitHub CLI first: https://cli.github.com"
			return m, nil
		}
		if m.ghStatus == gsync.GhNotAuthenticated {
			// Can't run interactive gh auth inside bubbletea alt screen,
			// so we tell the user to run it themselves
			m.errMsg = "Run 'gh auth login' in another terminal, then press enter to retry"
			return m, m.checkGh()
		}

	case setupStepChoose:
		m.setupStep = setupStepInput
		m.setupInput.Reset()
		if m.setupMethod == 0 {
			m.setupInput.Placeholder = "https://github.com/username/dotfiles.git"
		} else {
			m.setupInput.Placeholder = "dotfiles (creates username/dotfiles)"
		}
		m.setupInput.Focus()
		m.errMsg = ""
		return m, m.setupInput.Focus()

	case setupStepInput:
		val := strings.TrimSpace(m.setupInput.Value())
		if val == "" {
			m.errMsg = "Please enter a value"
			return m, nil
		}

		if m.setupMethod == 0 {
			// Existing repo URL
			m.cfg.RepoURL = val
			if err := m.cfg.Save(); err != nil {
				m.errMsg = fmt.Sprintf("Error saving config: %v", err)
				return m, nil
			}
			m.setupStep = setupStepWorking
			m.statusMsg = "Cloning repository..."
			m.errMsg = ""
			return m, m.cloneRepo()
		}

		// Create new repo
		m.setupStep = setupStepWorking
		m.statusMsg = "Creating repository..."
		m.errMsg = ""
		return m, m.createRepo(val)
	}

	return m, nil
}

func (m Model) checkGh() tea.Cmd {
	return func() tea.Msg {
		return ghCheckDoneMsg{status: gsync.CheckGh()}
	}
}

func (m Model) createRepo(name string) tea.Cmd {
	return func() tea.Msg {
		url, err := gsync.CreateGitHubRepo(name)
		return repoCreateDoneMsg{url: url, err: err}
	}
}

func (m Model) cloneRepo() tea.Cmd {
	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return repoCloneDoneMsg{err: err}
	}
}

func (m Model) viewSetup() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔧 DFC Setup"))
	b.WriteString("\n\n")
	b.WriteString("DFC backs up your dotfiles to a GitHub repository so you can\n")
	b.WriteString("keep your configurations in sync across multiple machines.\n\n")

	switch m.setupStep {
	case setupStepGhCheck:
		switch m.ghStatus {
		case gsync.GhChecking:
			b.WriteString("Checking for GitHub CLI...")
		case gsync.GhNotInstalled:
			b.WriteString(errorStyle.Render("✗ GitHub CLI (gh) is not installed"))
			b.WriteString("\n\n")
			b.WriteString("DFC uses the GitHub CLI to handle authentication.\n")
			b.WriteString("Install it from: ")
			b.WriteString(selectedStyle.Render("https://cli.github.com"))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Install gh, then restart dfc"))
		case gsync.GhNotAuthenticated:
			b.WriteString(warningStyle.Render("⚠ GitHub CLI is installed but not logged in"))
			b.WriteString("\n\n")
			b.WriteString("Run this in another terminal:\n\n")
			b.WriteString(selectedStyle.Render("  gh auth login"))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("enter retry • esc quit"))
		}

	case setupStepChoose:
		b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
		b.WriteString("\n\n")
		b.WriteString("Choose how to set up your dotfiles repository:\n\n")

		methods := []string{
			"Use an existing GitHub repository",
			"Create a new private repository",
		}
		for i, method := range methods {
			if i == m.setupMethod {
				b.WriteString(selectedStyle.Render("▸ " + method))
			} else {
				b.WriteString(normalStyle.Render("  " + method))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ select • enter confirm • esc quit"))

	case setupStepInput:
		if m.setupMethod == 0 {
			b.WriteString("Enter your repository URL:\n\n")
			b.WriteString(helpStyle.Render("Example: https://github.com/username/dotfiles.git"))
			b.WriteString("\n\n")
		} else {
			b.WriteString("Enter a name for your new repository:\n\n")
			b.WriteString(helpStyle.Render("Example: dotfiles (creates a private repo on your account)"))
			b.WriteString("\n\n")
		}
		b.WriteString(m.setupInput.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter confirm • esc back"))

	case setupStepWorking:
		b.WriteString(m.statusMsg)
	}

	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	return boxStyle.Render(b.String())
}

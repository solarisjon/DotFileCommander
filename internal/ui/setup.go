package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

// setupStep constants for clarity
const (
	setupStepGhCheck = 0 // checking gh status
	setupStepChoose  = 1 // choose: existing URL or create new (huh form)
	setupStepWorking = 2 // creating repo / cloning
)

type ghCheckDoneMsg struct{ status gsync.GhStatus }
type ghAuthDoneMsg struct{ err error }
type repoCreateDoneMsg struct {
	url string
	err error
}
type repoCloneDoneMsg struct{ err error }

func (m *Model) buildSetupForm() tea.Cmd {
	m.setupForm = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("method").
				Title("Repository Setup").
				Description("Choose how to set up your dotfiles repository").
				Options(
					huh.NewOption("Use an existing repository URL", "existing"),
					huh.NewOption("Create a new private repository (via gh)", "create"),
				).
				Value(&m.setupChoice),
			huh.NewInput().
				Key("value").
				Title("Repository").
				DescriptionFunc(func() string {
					if m.setupChoice == "create" {
						return "Name for your new repo (e.g. dotfiles)"
					}
					return "Full repository URL"
				}, &m.setupChoice).
				PlaceholderFunc(func() string {
					if m.setupChoice == "create" {
						return "dotfiles"
					}
					return "https://github.com/username/dotfiles.git"
				}, &m.setupChoice).
				Value(&m.setupValue),
		),
	).WithWidth(m.contentWidth()).
		WithShowHelp(false).
		WithShowErrors(true).
		WithTheme(dfcHuhTheme())
	return m.setupForm.Init()
}

func (m Model) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case ghCheckDoneMsg:
		m.ghStatus = msg.status
		if msg.status == gsync.GhReady {
			_ = gsync.SetupGitCredentialHelper()
			m.setupStep = setupStepChoose
			cmd := m.buildSetupForm()
			return m, cmd
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
		cmd := m.buildSetupForm()
		return m, cmd

	case repoCreateDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to create repo: %v", msg.err)
			m.setupStep = setupStepChoose
			cmd := m.buildSetupForm()
			return m, cmd
		}
		m.cfg.RepoURL = msg.url
		if err := m.cfg.Save(); err != nil {
			m.errMsg = fmt.Sprintf("Error saving config: %v", err)
			m.setupStep = setupStepChoose
			cmd := m.buildSetupForm()
			return m, cmd
		}
		m.statusMsg = "Repository created!"
		m.setupStep = setupStepWorking
		return m, m.cloneRepo()

	case repoCloneDoneMsg:
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to clone repo: %v", msg.err)
			m.setupStep = setupStepChoose
			cmd := m.buildSetupForm()
			return m, cmd
		}
		m.errMsg = ""
		m.statusMsg = ""
		m.currentView = viewMainMenu
		return m, nil

	case tea.KeyMsg:
		if m.setupStep == setupStepGhCheck {
			switch msg.String() {
			case "esc":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				return m.handleSetupGhEnter()
			}
			return m, nil
		}

		// For choose step, intercept esc
		if msg.String() == "esc" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Forward to huh form for choose step
	if m.setupStep == setupStepChoose && m.setupForm != nil {
		form, cmd := m.setupForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.setupForm = f
		}

		if m.setupForm.State == huh.StateCompleted {
			val := strings.TrimSpace(m.setupValue)
			if val == "" {
				m.errMsg = "Please enter a value"
				initCmd := m.buildSetupForm()
				return m, initCmd
			}

			if m.setupChoice == "existing" {
				m.cfg.RepoURL = val
				if err := m.cfg.Save(); err != nil {
					m.errMsg = fmt.Sprintf("Error saving config: %v", err)
					initCmd := m.buildSetupForm()
					return m, initCmd
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

		return m, cmd
	}

	return m, nil
}

func (m Model) handleSetupGhEnter() (tea.Model, tea.Cmd) {
	if m.ghStatus == gsync.GhNotInstalled {
		m.errMsg = "Please install the GitHub CLI first: https://cli.github.com"
		return m, nil
	}
	if m.ghStatus == gsync.GhNotAuthenticated {
		m.errMsg = "Run 'gh auth login' in another terminal, then press enter to retry"
		return m, m.checkGh()
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

	b.WriteString(sectionHeader("🔧", "DFC Setup"))
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
		case gsync.GhReady:
			b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
		}

	case setupStepChoose:
		b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
		b.WriteString("\n\n")
		if m.setupForm != nil {
			b.WriteString(m.setupForm.View())
		}

	case setupStepWorking:
		b.WriteString(m.statusMsg)
	}

	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
	}

	return m.box().Render(b.String())
}

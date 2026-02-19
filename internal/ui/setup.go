package ui

import (
"fmt"
"strings"

"github.com/charmbracelet/bubbles/textinput"
tea "github.com/charmbracelet/bubbletea"
gsync "github.com/solarisjon/dfc/internal/sync"
)

// setupStep constants
const (
setupStepGhCheck   = 0 // checking gh status
setupStepGitID     = 1 // checking/setting git identity
setupStepChoose    = 2 // choose: existing URL or create new
setupStepInput     = 3 // enter URL or repo name
setupStepWorking   = 4 // creating repo / cloning
)

type ghCheckDoneMsg struct{ status gsync.GhStatus }
type ghAuthDoneMsg struct{ err error }
type gitIDCheckMsg struct{ id gsync.GitIdentity }
type gitIDSetMsg struct{ err error }
type repoCreateDoneMsg struct {
url string
err error
}
type repoCloneDoneMsg struct{ err error }

func (m *Model) initSetupInput() {
ti := textinput.New()
ti.Placeholder = "https://github.com/username/dotfiles.git"
ti.CharLimit = 256
ti.Width = m.contentWidth() - 4
m.setupInput = ti
}

func (m Model) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
switch msg := msg.(type) {

case ghCheckDoneMsg:
m.ghStatus = msg.status
if msg.status == gsync.GhReady {
_ = gsync.SetupGitCredentialHelper()
// Check git identity before proceeding
m.setupStep = setupStepGitID
return m, m.checkGitID()
}
return m, nil

case ghAuthDoneMsg:
if msg.err != nil {
m.errMsg = fmt.Sprintf("Authentication failed: %v", msg.err)
return m, nil
}
_ = gsync.SetupGitCredentialHelper()
m.ghStatus = gsync.GhReady
m.setupStep = setupStepGitID
m.errMsg = ""
return m, m.checkGitID()

case gitIDCheckMsg:
m.gitID = msg.id
if msg.id.Name != "" && msg.id.Email != "" {
// Identity configured, skip to repo choice
m.setupStep = setupStepChoose
return m, nil
}
// Need user input — initialize fields
m.initGitIDInputs()
return m, nil

case gitIDSetMsg:
if msg.err != nil {
m.errMsg = fmt.Sprintf("Failed to set git identity: %v", msg.err)
return m, nil
}
m.gitID.Name = strings.TrimSpace(m.gitNameIn.Value())
m.gitID.Email = strings.TrimSpace(m.gitEmailIn.Value())
m.errMsg = ""
m.setupStep = setupStepChoose
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
if m.setupStep == setupStepInput {
m.setupStep = setupStepChoose
m.errMsg = ""
return m, nil
}
if m.setupStep == setupStepGitID {
m.setupStep = setupStepChoose
m.errMsg = ""
return m, nil
}
if m.cfg.IsConfigured() {
m.currentView = viewMainMenu
return m, nil
}
m.quitting = true
return m, tea.Quit

case "enter":
return m.handleSetupEnter()

case "up", "k":
if m.setupStep == setupStepChoose && m.setupMethod > 0 {
m.setupMethod--
}
case "down", "j":
if m.setupStep == setupStepChoose && m.setupMethod < 1 {
m.setupMethod++
}
case "tab":
if m.setupStep == setupStepGitID {
m.gitIDField = (m.gitIDField + 1) % 2
if m.gitIDField == 0 {
m.gitNameIn.Focus()
m.gitEmailIn.Blur()
} else {
m.gitNameIn.Blur()
m.gitEmailIn.Focus()
}
return m, nil
}
case "shift+tab":
if m.setupStep == setupStepGitID {
m.gitIDField = (m.gitIDField + 1) % 2
if m.gitIDField == 0 {
m.gitNameIn.Focus()
m.gitEmailIn.Blur()
} else {
m.gitNameIn.Blur()
m.gitEmailIn.Focus()
}
return m, nil
}
}
}

if m.setupStep == setupStepInput {
var cmd tea.Cmd
m.setupInput, cmd = m.setupInput.Update(msg)
return m, cmd
}

if m.setupStep == setupStepGitID {
var cmd tea.Cmd
if m.gitIDField == 0 {
m.gitNameIn, cmd = m.gitNameIn.Update(msg)
} else {
m.gitEmailIn, cmd = m.gitEmailIn.Update(msg)
}
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
m.errMsg = "Run 'gh auth login' in another terminal, then press enter to retry"
return m, m.checkGh()
}

case setupStepGitID:
name := strings.TrimSpace(m.gitNameIn.Value())
email := strings.TrimSpace(m.gitEmailIn.Value())
if name == "" || email == "" {
m.errMsg = "Both name and email are required"
return m, nil
}
m.errMsg = ""
return m, m.setGitID(name, email)

case setupStepChoose:
m.setupStep = setupStepInput
m.initSetupInput()
if m.setupMethod == 0 {
// Pre-fill with current URL if configured
if m.cfg.RepoURL != "" {
m.setupInput.SetValue(m.cfg.RepoURL)
}
m.setupInput.Placeholder = "https://github.com/username/dotfiles.git"
} else {
m.setupInput.Placeholder = "dotfiles"
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

func (m Model) checkGitID() tea.Cmd {
return func() tea.Msg {
return gitIDCheckMsg{id: gsync.CheckGitIdentity()}
}
}

func (m Model) setGitID(name, email string) tea.Cmd {
return func() tea.Msg {
return gitIDSetMsg{err: gsync.SetGitIdentity(name, email)}
}
}

func (m *Model) initGitIDInputs() {
ti := textinput.New()
ti.Placeholder = "Your Name"
ti.CharLimit = 128
ti.Width = m.contentWidth() - 4
if m.gitID.Name != "" {
ti.SetValue(m.gitID.Name)
}
ti.Focus()
m.gitNameIn = ti

ei := textinput.New()
ei.Placeholder = "you@example.com"
ei.CharLimit = 128
ei.Width = m.contentWidth() - 4
if m.gitID.Email != "" {
ei.SetValue(m.gitID.Email)
}
m.gitEmailIn = ei

m.gitIDField = 0
}

func (m Model) viewSetup() string {
var b strings.Builder

b.WriteString(sectionHeader("🔧", "DFC Setup"))
b.WriteString("\n\n")

// Show current config if re-entering from Settings
if m.cfg.IsConfigured() {
b.WriteString(helpStyle.Render("Current repo: "))
b.WriteString(selectedStyle.Render(m.cfg.RepoURL))
b.WriteString("\n\n")
} else {
b.WriteString("DFC backs up your dotfiles to a GitHub repository so you can\n")
b.WriteString("keep your configurations in sync across multiple machines.\n\n")
}

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
b.WriteString(statusBar("esc back"))
case gsync.GhNotAuthenticated:
b.WriteString(warningStyle.Render("⚠ GitHub CLI is installed but not logged in"))
b.WriteString("\n\n")
b.WriteString("Run this in another terminal:\n\n")
b.WriteString(selectedStyle.Render("  gh auth login"))
b.WriteString("\n\n")
b.WriteString(statusBar("enter retry • esc back"))
case gsync.GhReady:
b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
}

case setupStepGitID:
b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
b.WriteString("\n\n")
b.WriteString("Git needs to know who you are for commits.\n")
b.WriteString("Enter your name and email:\n\n")

nameLabel := "  Name:  "
emailLabel := "  Email: "
if m.gitIDField == 0 {
nameLabel = selectedStyle.Render("▸ ") + "Name:  "
} else {
emailLabel = selectedStyle.Render("▸ ") + "Email: "
}
b.WriteString(nameLabel)
b.WriteString(m.gitNameIn.View())
b.WriteString("\n")
b.WriteString(emailLabel)
b.WriteString(m.gitEmailIn.View())
b.WriteString("\n\n")
b.WriteString(statusBar("tab switch • enter confirm • esc skip"))

case setupStepChoose:
b.WriteString(successStyle.Render("✓ GitHub CLI authenticated"))
b.WriteString("\n")
b.WriteString(successStyle.Render(fmt.Sprintf("✓ Git identity: %s <%s>", m.gitID.Name, m.gitID.Email)))
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
b.WriteString(statusBar("↑/↓ select • enter confirm • esc back"))

case setupStepInput:
if m.setupMethod == 0 {
b.WriteString("Enter your repository URL:\n\n")
} else {
b.WriteString("Enter a name for your new repository:\n\n")
}
b.WriteString(m.setupInput.View())
b.WriteString("\n\n")
b.WriteString(statusBar("enter confirm • esc back"))

case setupStepWorking:
b.WriteString(m.statusMsg)
}

if m.errMsg != "" {
b.WriteString("\n\n")
b.WriteString(errorStyle.Render("✗ " + m.errMsg))
}

return m.box().Render(b.String())
}

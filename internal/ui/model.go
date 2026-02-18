package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/backup"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/manifest"
	"github.com/solarisjon/dfc/internal/restore"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

type view int

const (
	viewSetup view = iota
	viewMainMenu
	viewEntryList
	viewAddEntry
	viewBackup
	viewRestore
	viewConfigBrowser
	viewRemote
	viewReset
	viewProfileEdit
)

// Model is the root bubbletea model.
type Model struct {
	cfg         *config.Config
	currentView view
	width       int
	height      int

	// Main menu
	menuItems    []string
	menuCursor   int

	// Entry list
	entryCursor  int

	// Add entry
	addInput     textinput.Model
	addStep      int // 0=path, 1=name, 2=profile-specific
	addNameInput textinput.Model
	addIsDir     bool

	// Config browser
	browserDirs   []browserItem
	browserCursor int

	// Setup
	setupInput   textinput.Model
	setupStep    int // setupStep* constants
	setupMethod  int // 0=existing URL, 1=create via gh
	ghStatus     gsync.GhStatus

	// Backup/Restore progress
	progressItems    []progressItem
	progressDone     bool
	statusMsg        string
	backupCh         <-chan backup.Progress
	backupConflicts  []string // entry paths that were updated remotely
	backupConfirmed  bool

	// Restore selection
	restoreStep      int
	restoreCursor    int
	restoreEntries   []restoreEntryItem
	restoreCh        <-chan restore.Progress
	restoreManifest  *manifest.Manifest
	restoreConfirmed bool

	// Error display
	errMsg string

	// Remote view
	remoteEntries []remoteEntry
	remoteSyncing bool

	// Reset view
	resetStep      int
	resetConfirmed bool

	// Profile edit
	profileInput   textinput.Model
	profileReturn  view // view to return to after profile edit

	// Add entry — profile toggle
	addProfileSpecific bool

	quitting bool
}

type progressItem struct {
	name        string
	done        bool
	err         error
	percent     float64
	contentHash string
}

// New creates a new root model.
func New(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "git@github.com:user/dotfiles.git"
	ti.CharLimit = 256
	ti.Width = 60

	addTi := textinput.New()
	addTi.Placeholder = "~/.config/kitty"
	addTi.CharLimit = 256
	addTi.Width = 60

	nameTi := textinput.New()
	nameTi.Placeholder = "Kitty Terminal"
	nameTi.CharLimit = 100
	nameTi.Width = 40

	profileTi := textinput.New()
	profileTi.Placeholder = "work"
	profileTi.CharLimit = 50
	profileTi.Width = 30

	startView := viewMainMenu
	if !cfg.IsConfigured() {
		startView = viewSetup
	}

	// Check gh status synchronously before building model
	var ghSt gsync.GhStatus
	if startView == viewSetup {
		ghSt = gsync.CheckGh()
	}

	initialStep := setupStepGhCheck
	if ghSt == gsync.GhReady {
		_ = gsync.SetupGitCredentialHelper()
		initialStep = setupStepChoose
	}

	return Model{
		cfg:         cfg,
		currentView: startView,
		menuItems:   []string{"Backup", "Restore", "Manage Entries", "Remote Status", "Reset", "Device Profile", "Settings"},
		setupInput:  ti,
		addInput:    addTi,
		addNameInput: nameTi,
		profileInput: profileTi,
		ghStatus:    ghSt,
		setupStep:   initialStep,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case backupProgressMsg:
		return m.handleBackupProgress(msg)
	case repoSyncDoneMsg:
		return m.handleRepoSyncDone(msg)
	case restoreProgressMsg:
		return m.handleRestoreProgress(msg)
	case restoreSyncDoneMsg:
		return m.handleRestoreSyncDone(msg)
	case ghCheckDoneMsg:
		return m.updateSetup(msg)
	case ghAuthDoneMsg:
		return m.updateSetup(msg)
	case repoCreateDoneMsg:
		return m.updateSetup(msg)
	case repoCloneDoneMsg:
		return m.updateSetup(msg)
	case remoteViewSyncMsg:
		return m.updateRemoteView(msg)
	}

	switch m.currentView {
	case viewSetup:
		return m.updateSetup(msg)
	case viewMainMenu:
		return m.updateMainMenu(msg)
	case viewEntryList:
		return m.updateEntryList(msg)
	case viewAddEntry:
		return m.updateAddEntry(msg)
	case viewBackup:
		return m.updateBackupView(msg)
	case viewRestore:
		return m.updateRestoreView(msg)
	case viewConfigBrowser:
		return m.updateConfigBrowser(msg)
	case viewRemote:
		return m.updateRemoteView(msg)
	case viewReset:
		return m.updateResetView(msg)
	case viewProfileEdit:
		return m.updateProfileEdit(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.currentView {
	case viewSetup:
		return m.viewSetup()
	case viewMainMenu:
		return m.viewMainMenu()
	case viewEntryList:
		return m.viewEntryList()
	case viewAddEntry:
		return m.viewAddEntry()
	case viewBackup:
		return m.viewBackupProgress()
	case viewRestore:
		return m.viewRestoreProgress()
	case viewConfigBrowser:
		return m.viewConfigBrowser()
	case viewRemote:
		return m.viewRemoteView()
	case viewReset:
		return m.viewResetView()
	case viewProfileEdit:
		return m.viewProfileEdit()
	}

	return ""
}

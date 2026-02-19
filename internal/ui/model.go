package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
	entryList    *list.Model

	// Add entry (huh form)
	addForm            *huh.Form
	addPath            string
	addName            string
	addStep            int // 0=path phase, 1=name+profile phase
	addIsDir           bool
	addProfileSpecific bool

	// Config browser
	browserDirs   []browserItem
	browserCursor int

	// Setup
	setupStep    int // setupStep* constants
	setupForm    *huh.Form
	setupChoice  string // "existing" or "create"
	setupValue   string // URL or repo name
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
	remoteTable   *table.Model

	// Reset view
	resetStep      int
	resetConfirmed bool
	resetType      int

	// Profile edit
	profileInput   textinput.Model
	profileReturn  view // view to return to after profile edit

	quitting bool
}

type progressItem struct {
	name        string
	done        bool
	err         error
	percent     float64
	contentHash string
	skipped     int
	warning     string
}

const (
	minBoxWidth = 60
	maxBoxWidth = 120
	// border (2) + padding (3*2) = 8 chars of chrome
	boxChrome = 8
)

// box returns boxStyle sized to the current terminal width.
func (m Model) box() lipgloss.Style {
	w := m.width
	if w < minBoxWidth {
		w = minBoxWidth
	}
	if w > maxBoxWidth {
		w = maxBoxWidth
	}
	return boxStyle.Width(w)
}

// contentWidth returns usable character width inside the box.
func (m Model) contentWidth() int {
	w := m.width - boxChrome
	if w < minBoxWidth-boxChrome {
		w = minBoxWidth - boxChrome
	}
	if w > maxBoxWidth-boxChrome {
		w = maxBoxWidth - boxChrome
	}
	return w
}

// New creates a new root model.
func New(cfg *config.Config) Model {
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
	case resetNukeDoneMsg:
		return m.updateResetView(msg)
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

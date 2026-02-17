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
	viewTagEdit
	viewBackup
	viewRestore
	viewConfigBrowser
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
	addStep      int // 0=path, 1=name, 2=tags
	addNameInput textinput.Model
	addTagInput  textinput.Model
	addIsDir     bool

	// Tag edit
	tagEditIdx   int
	tagInput     textinput.Model

	// Config browser
	browserDirs     []browserItem
	browserCursor   int
	browserStep     int // 0=tags, 1=select
	browserTagInput textinput.Model

	// Setup
	setupInput   textinput.Model
	setupStep    int // setupStep* constants
	setupMethod  int // 0=existing URL, 1=create via gh
	ghStatus     gsync.GhStatus

	// Backup/Restore progress
	progressItems []progressItem
	progressDone  bool
	statusMsg     string
	backupCh      <-chan backup.Progress

	// Restore selection
	restoreStep     int // restoreStep* constants
	restoreCursor   int
	restoreTags     []restoreTagItem
	restoreAllTags  bool
	restoreEntries  []restoreEntryItem
	restoreCh       <-chan restore.Progress
	restoreManifest *manifest.Manifest

	// Error display
	errMsg string

	quitting bool
}

type progressItem struct {
	name    string
	done    bool
	err     error
	percent float64
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

	tagTi := textinput.New()
	tagTi.Placeholder = "home, work"
	tagTi.CharLimit = 200
	tagTi.Width = 40

	tagEditTi := textinput.New()
	tagEditTi.Placeholder = "home, work"
	tagEditTi.CharLimit = 200
	tagEditTi.Width = 40

	browserTagTi := textinput.New()
	browserTagTi.Placeholder = "home, work, laptop"
	browserTagTi.CharLimit = 200
	browserTagTi.Width = 40

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
		menuItems:   []string{"Backup", "Restore", "Manage Entries", "Settings"},
		setupInput:  ti,
		addInput:    addTi,
		addNameInput: nameTi,
		addTagInput: tagTi,
		tagInput:    tagEditTi,
		browserTagInput: browserTagTi,
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
	case viewTagEdit:
		return m.updateTagEdit(msg)
	case viewBackup:
		return m.updateBackupView(msg)
	case viewRestore:
		return m.updateRestoreView(msg)
	case viewConfigBrowser:
		return m.updateConfigBrowser(msg)
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
	case viewTagEdit:
		return m.viewTagEdit()
	case viewBackup:
		return m.viewBackupProgress()
	case viewRestore:
		return m.viewRestoreProgress()
	case viewConfigBrowser:
		return m.viewConfigBrowser()
	}

	return ""
}

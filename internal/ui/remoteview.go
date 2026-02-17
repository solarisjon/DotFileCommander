package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/hash"
	"github.com/solarisjon/dfc/internal/manifest"
	gsync "github.com/solarisjon/dfc/internal/sync"
)

type remoteViewSyncMsg struct{ err error }

func (m Model) updateRemoteView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case remoteViewSyncMsg:
		m.remoteSyncing = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.loadRemoteData()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.currentView = viewMainMenu
		}
	}
	return m, nil
}

type remoteEntry struct {
	name          string
	path          string
	tags          string
	tagsPlain     int
	repoVer       int
	localVer      int
	updatedBy     string
	isLocal       bool // exists in local config
	isRemote      bool // exists in remote manifest
	localModified bool // local content differs from last known hash
}

func (m *Model) initRemoteView() tea.Cmd {
	m.remoteSyncing = true
	m.remoteEntries = nil
	m.errMsg = ""
	return func() tea.Msg {
		err := gsync.EnsureRepo(m.cfg.RepoURL, m.cfg.RepoPath)
		return remoteViewSyncMsg{err: err}
	}
}

func (m *Model) loadRemoteData() {
	mf, err := manifest.Load(m.cfg.RepoPath)
	if err != nil {
		m.errMsg = err.Error()
		return
	}

	// Build lookup of local entries by path
	localByPath := make(map[string]*localEntryInfo)
	for _, e := range m.cfg.Entries {
		localByPath[e.Path] = &localEntryInfo{
			name:     e.Name,
			tags:     e.Tags,
			localVer: e.LocalVersion,
			lastHash: e.LastHash,
			isDir:    e.IsDir,
		}
	}

	var entries []remoteEntry

	// Add all entries from manifest (remote)
	for path, ev := range mf.Entries {
		re := remoteEntry{
			path:      path,
			repoVer:   ev.Version,
			updatedBy: ev.UpdatedBy,
			isRemote:  true,
		}
		if local, ok := localByPath[path]; ok {
			re.isLocal = true
			re.name = local.name
			re.localVer = local.localVer
			if len(local.tags) > 0 {
				re.tags = strings.Join(local.tags, ", ")
				re.tagsPlain = len(re.tags)
			}
			// Detect local modifications via hash comparison
			if local.lastHash != "" {
				currentHash, err := hash.HashEntry(findConfigEntry(m.cfg, path))
				if err == nil && currentHash != local.lastHash {
					re.localModified = true
				}
			}
			delete(localByPath, path) // mark as seen
		} else {
			re.name = entry.FriendlyName(path)
		}
		if re.name == "" {
			re.name = entry.FriendlyName(path)
		}
		entries = append(entries, re)
	}

	// Add local-only entries (not in manifest yet — never backed up)
	for path, local := range localByPath {
		re := remoteEntry{
			path:     path,
			name:     local.name,
			localVer: local.localVer,
			isLocal:  true,
			isRemote: false,
		}
		if re.name == "" {
			re.name = entry.FriendlyName(path)
		}
		if len(local.tags) > 0 {
			re.tags = strings.Join(local.tags, ", ")
			re.tagsPlain = len(re.tags)
		}
		entries = append(entries, re)
	}

	m.remoteEntries = entries
}

type localEntryInfo struct {
	name     string
	tags     []string
	localVer int
	lastHash string
	isDir    bool
}

func (m Model) viewRemoteView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🌐 Remote Repository Status"))
	b.WriteString("\n\n")

	if m.remoteSyncing {
		b.WriteString("Syncing repository...\n")
		b.WriteString(helpStyle.Render("esc back"))
		return boxStyle.Render(b.String())
	}

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ "+m.errMsg))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return boxStyle.Render(b.String())
	}

	if len(m.remoteEntries) == 0 {
		b.WriteString(helpStyle.Render("No entries found in remote or local config."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return boxStyle.Render(b.String())
	}

	// Compute column widths
	maxName := 4 // "Name"
	maxPath := 4 // "Path"
	for _, re := range m.remoteEntries {
		if len(re.name) > maxName {
			maxName = len(re.name)
		}
		if len(re.path) > maxPath {
			maxPath = len(re.path)
		}
	}

	// Header
	header := fmt.Sprintf("  %s  %s  %s  %s  %s",
		padRight("Name", maxName),
		padRight("Path", maxPath),
		padRight("Remote", 8),
		padRight("Local", 8),
		"Status",
	)
	b.WriteString(secondaryStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(strings.Repeat("─", len(header))))
	b.WriteString("\n")

	for _, re := range m.remoteEntries {
		nameCol := padRight(re.name, maxName)
		pathCol := helpStyle.Render(padRight(re.path, maxPath))

		var remoteCol, localCol, status string

		if re.isRemote {
			remoteCol = padRight(fmt.Sprintf("v%d", re.repoVer), 8)
		} else {
			remoteCol = helpStyle.Render(padRight("—", 8))
		}

		if re.isLocal && re.localVer > 0 {
			localCol = padRight(fmt.Sprintf("v%d", re.localVer), 8)
		} else if re.isLocal {
			localCol = helpStyle.Render(padRight("v0", 8))
		} else {
			localCol = helpStyle.Render(padRight("—", 8))
		}

		switch {
		case !re.isLocal && re.isRemote:
			status = warningStyle.Render("⚠ not tracked locally")
		case re.isLocal && !re.isRemote:
			status = helpStyle.Render("⊘ never backed up")
		case re.localModified && re.localVer < re.repoVer:
			status = errorStyle.Render("⚡ conflict (local modified + repo newer)")
		case re.localModified:
			status = warningStyle.Render("⚠ modified locally — backup recommended")
		case re.localVer < re.repoVer:
			status = warningStyle.Render(fmt.Sprintf("⬇ outdated (from %s)", re.updatedBy))
		case re.localVer == re.repoVer && re.repoVer > 0:
			status = successStyle.Render("✓ current")
		default:
			status = helpStyle.Render("—")
		}

		line := fmt.Sprintf("  %s  %s  %s  %s  %s", nameCol, pathCol, remoteCol, localCol, status)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc back"))

	return boxStyle.Render(b.String())
}

// findConfigEntry returns a config.Entry for the given path, used for hashing.
func findConfigEntry(cfg *config.Config, path string) config.Entry {
	for _, e := range cfg.Entries {
		if e.Path == path {
			return e
		}
	}
	return config.Entry{Path: path}
}

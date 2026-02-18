package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/hash"
	"github.com/solarisjon/dfc/internal/manifest"
	"github.com/solarisjon/dfc/internal/storage"
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
	name            string
	path            string
	tags            string
	tagsPlain       int
	repoVer         int
	localVer        int
	updatedBy       string
	isLocal         bool // exists in local config
	isRemote        bool // exists in remote manifest
	localModified   bool // local content differs from last known hash
	profileSpecific bool // entry is profile-specific
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

	var entries []remoteEntry

	// Track which manifest keys we've seen
	seenKeys := make(map[string]bool)

	// Build entries from local config, looking up their manifest keys
	for _, e := range m.cfg.Entries {
		mkey := storage.ManifestKey(e, m.cfg.DeviceProfile)
		seenKeys[mkey] = true
		ev := mf.GetEntry(mkey)

		re := remoteEntry{
			path:      e.Path,
			name:      e.Name,
			repoVer:   ev.Version,
			updatedBy: ev.UpdatedBy,
			isRemote:  ev.Version > 0,
			isLocal:   true,
			localVer:  e.LocalVersion,
		}
		if re.name == "" {
			re.name = entry.FriendlyName(e.Path)
		}
		if len(e.Tags) > 0 {
			re.tags = strings.Join(e.Tags, ", ")
			re.tagsPlain = len(re.tags)
		}
		if e.ProfileSpecific {
			re.profileSpecific = true
		}
		// Detect local modifications via hash comparison
		if e.LastHash != "" {
			currentHash, hashErr := hash.HashEntry(e)
			if hashErr == nil && currentHash != e.LastHash {
				re.localModified = true
			}
		}
		entries = append(entries, re)
	}

	// Add manifest entries not in local config (from other profiles or untracked)
	for mkey, ev := range mf.Entries {
		if seenKeys[mkey] {
			continue
		}
		// Extract the display path from the manifest key
		displayPath := manifestKeyToPath(mkey)
		re := remoteEntry{
			path:      displayPath,
			name:      entry.FriendlyName(displayPath),
			repoVer:   ev.Version,
			updatedBy: ev.UpdatedBy,
			isRemote:  true,
			isLocal:   false,
		}
		entries = append(entries, re)
	}

	m.remoteEntries = entries
}

// manifestKeyToPath extracts the original entry path from a manifest key.
// "shared/~/.bashrc" → "~/.bashrc"
// "profiles/work/~/.config/claude" → "~/.config/claude"
func manifestKeyToPath(key string) string {
	if strings.HasPrefix(key, "shared/") {
		return key[len("shared/"):]
	}
	if strings.HasPrefix(key, "profiles/") {
		// profiles/<name>/<path>
		rest := key[len("profiles/"):]
		idx := strings.Index(rest, "/")
		if idx >= 0 {
			return rest[idx+1:]
		}
	}
	return key // legacy key format
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

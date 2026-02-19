package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			m.buildRemoteTable()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.currentView = viewMainMenu
			return m, nil
		}
	}
	// Forward to the table for scrolling/navigation
	if m.remoteTable != nil {
		t, cmd := m.remoteTable.Update(msg)
		m.remoteTable = &t
		return m, cmd
	}
	return m, nil
}

func (m *Model) buildRemoteTable() {
	cw := m.contentWidth()

	// Compute proportional column widths
	fixedW := 8 + 8 + 22 + 6 // Remote + Local + Status + spacing
	flexW := cw - fixedW
	if flexW < 20 {
		flexW = 20
	}
	nameW := flexW * 2 / 5
	pathW := flexW - nameW

	cols := []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Path", Width: pathW},
		{Title: "Remote", Width: 8},
		{Title: "Local", Width: 8},
		{Title: "Status", Width: 22},
	}

	rows := make([]table.Row, len(m.remoteEntries))
	for i, re := range m.remoteEntries {
		var remoteStr, localStr, status string

		if re.isRemote {
			remoteStr = fmt.Sprintf("v%d", re.repoVer)
		} else {
			remoteStr = "—"
		}

		if re.isLocal && re.localVer > 0 {
			localStr = fmt.Sprintf("v%d", re.localVer)
		} else if re.isLocal {
			localStr = "v0"
		} else {
			localStr = "—"
		}

		switch {
		case !re.isLocal && re.isRemote:
			status = "⚠ not tracked locally"
		case re.isLocal && !re.isRemote:
			status = "⊘ never backed up"
		case re.localModified && re.localVer < re.repoVer:
			status = "⚡ conflict"
		case re.localModified:
			status = "⚠ modified locally"
		case re.localVer < re.repoVer:
			status = "⬇ outdated"
		case re.localVer == re.repoVer && re.repoVer > 0:
			status = "✓ current"
		default:
			status = "—"
		}

		rows[i] = table.Row{re.name, re.path, remoteStr, localStr, status}
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(dimColor).
		BorderBottom(true).
		Bold(true).
		Foreground(secondaryColor)
	s.Selected = s.Selected.
		Foreground(brightWhite).
		Background(lipgloss.Color("#3B0764")).
		Bold(true)
	s.Cell = s.Cell.Foreground(textColor)

	height := len(rows)
	if height > 15 {
		height = 15
	}
	if height < 3 {
		height = 3
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithStyles(s),
	)
	m.remoteTable = &t
}

type remoteEntry struct {
	name            string
	path            string
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

	b.WriteString(sectionHeader("🌐", "Remote Repository Status"))
	b.WriteString("\n\n")

	if m.remoteSyncing {
		b.WriteString("Syncing repository...\n")
		b.WriteString(helpStyle.Render("esc back"))
		return m.box().Render(b.String())
	}

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ "+m.errMsg))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return m.box().Render(b.String())
	}

	if len(m.remoteEntries) == 0 {
		b.WriteString(helpStyle.Render("No entries found in remote or local config."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return m.box().Render(b.String())
	}

	if m.remoteTable != nil {
		b.WriteString(m.remoteTable.View())
		b.WriteString("\n")

		// Color-coded status legend
		row := m.remoteTable.SelectedRow()
		if len(row) > 4 {
			detail := m.remoteStatusDetail(row)
			if detail != "" {
				b.WriteString("\n")
				b.WriteString(detail)
			}
		}
	}

	b.WriteString(statusBar("↑/↓ navigate • esc back"))

	return m.box().Render(b.String())
}

// remoteStatusDetail returns a color-styled detail line for the selected row.
func (m Model) remoteStatusDetail(row table.Row) string {
	status := row[4]
	switch {
	case strings.Contains(status, "conflict"):
		return errorStyle.Render("  ⚡ Both local and remote have changed — manual review needed")
	case strings.Contains(status, "modified"):
		return warningStyle.Render("  ⚠ Local changes detected — run Backup to push them")
	case strings.Contains(status, "outdated"):
		return warningStyle.Render("  ⬇ Remote is newer — run Restore to update")
	case strings.Contains(status, "not tracked"):
		return warningStyle.Render("  ⚠ Exists in repo but not in your local config")
	case strings.Contains(status, "never backed"):
		return helpStyle.Render("  ⊘ Not yet pushed to remote — run Backup")
	case strings.Contains(status, "current"):
		return successStyle.Render("  ✓ Local and remote are in sync")
	default:
		return ""
	}
}

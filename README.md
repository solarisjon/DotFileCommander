# DFC — Dot File Commander

A TUI application for backing up and restoring your dotfiles across multiple machines via a Git repository. Keep your configurations in sync with device profiles, content-hash conflict detection, and per-entry version tracking.

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux-lightgrey)

## Features

- **Backup & Restore** — Sync dotfiles to/from a Git repo with real-time progress bars
- **Device Profiles** — Per-machine identities (e.g. `work`, `home`) with profile-specific storage so the same config can differ between machines
- **Conflict Detection** — SHA256 content hashing detects remote changes before overwriting
- **Browse ~/.config** — File browser to quickly select config directories to track
- **Version Tracking** — Per-entry versioning shows which entries are outdated across machines
- **Symlink Support** — Symlinks are preserved during backup and restore, not followed
- **Graceful Error Handling** — Unreadable files, sockets, and pipes are skipped per-entry without aborting; entries with nothing to back up get descriptive warnings
- **Responsive UI** — Layout dynamically adapts to terminal width (60–120 chars)
- **Reset & Wipe** — Local reset or full remote repo wipe for clean-slate recovery
- **GitHub CLI Integration** — Uses `gh` for authentication and repo creation
- **TUI Interface** — Built with [Charm](https://charm.sh) libraries (bubbletea, bubbles, lipgloss, huh)
  - **Fuzzy-filterable entry list** — Type `/` to search entries by name or path
  - **Scrollable remote status table** — Navigable table view with color-coded sync state
  - **Interactive forms** — Polished input forms for setup and add-entry flows (huh)

## Installation

### Prerequisites

- **Go 1.24+**
- **GitHub CLI** (`gh`) — [Install from cli.github.com](https://cli.github.com)

### Quick install

```bash
git clone https://github.com/solarisjon/DotFileCommander.git
cd DotFileCommander
./install.sh
```

This builds the binary and installs it to `~/.local/bin/dfc`.

### Build from source

```bash
go build -o dfc ./cmd/dfc/
```

### GitHub CLI setup

DFC uses the GitHub CLI for authentication. If you haven't already:

```bash
gh auth login
```

DFC will detect `gh` and guide you through setup on first run.

## Usage

```bash
dfc
```

### First run

On first launch, DFC walks you through setup:

1. **GitHub CLI check** — Verifies `gh` is installed and authenticated
2. **Repository setup** — Enter an existing repo URL or create a new one via `gh`
3. **Device profile** — Set a profile name for this machine (e.g. `work`, `home`)
4. **Ready** — You're taken to the main menu

### Main menu

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate |
| `Enter` | Select |
| `q` | Quit |

**Options:**

- **⬆ Backup** — Back up all tracked entries to the repo
- **⬇ Restore** — Restore entries with version comparison
- **📋 Manage Entries** — Add, remove, and configure tracked dotfiles
- **🌐 Remote Status** — View sync state with the remote repo
- **🔄 Reset** — Local reset or full remote wipe
- **👤 Device Profile** — View or change this machine's profile
- **⚙ Settings** — Re-run setup wizard

### Managing entries

| Key | Action |
|-----|--------|
| `a` | Add a new entry (path → name → profile-specific?) |
| `b` | Browse `~/.config` directories to bulk-add |
| `d` | Delete selected entry |
| `p` | Toggle profile-specific on selected entry |
| `/` | Fuzzy filter entries by name or path |
| `Esc` | Back to main menu |

#### Browsing ~/.config

Press `b` from the entry list to open the config browser. Select directories with `Space`, `a` for all, `n` for none, then `Enter` to add. Already-tracked entries appear dimmed with a checkmark.

### Backup

Select **Backup** from the main menu. DFC will:

1. Sync the local repo clone
2. Copy each tracked entry into the repo (preserving symlinks, skipping `.git`)
3. Compute content hashes and bump versions in the manifest
4. Commit and push

Profile-specific entries are stored under `profiles/<profile>/`, shared entries under `shared/`.

### Restore

Select **Restore** from the main menu:

1. **Select entries** — Check which entries to restore, with version indicators:
   - `⬆ v1→v3` (amber) — repo has a newer version
   - `v3 ✓` (green) — up to date
   - 👤 icon for profile-specific entries
2. **Progress** — Files are restored with progress bars (symlinks preserved)

### Reset

Two options from the reset menu:

- **🧹 Local Reset** — Removes the local clone and clears config entries. Remote repo is untouched.
- **💣 Full Remote Wipe** — Destroys all files and history in the remote repo (force-push). Requires double confirmation. Useful for testing or clearing out-of-sync states.

## Configuration

Config is stored at `~/.config/dfc/config.yaml`:

```yaml
repo_url: https://github.com/user/dotfiles.git
repo_path: /Users/you/.config/dfc/repo
device_profile: work
entries:
  - path: ~/.config/kitty
    name: Kitty Terminal
    is_dir: true
    local_version: 3
    last_hash: a1b2c3...
  - path: ~/.config/claude
    name: Claude Code
    is_dir: true
    profile_specific: true
    local_version: 2
    last_hash: d4e5f6...
```

### Version manifest

A `.dfc-manifest.yaml` file in the repo tracks per-entry versions and content hashes:

```yaml
entries:
  shared/~/.config/kitty:
    version: 3
    hash: a1b2c3...
    updated_at: 2026-02-17T02:30:00Z
    updated_by: work-laptop
  profiles/work/~/.config/claude:
    version: 2
    hash: d4e5f6...
    updated_at: 2026-02-18T10:15:00Z
    updated_by: work-laptop
```

### Repo layout

```
repo/
├── .dfc-manifest.yaml
├── shared/                    # Entries shared across all devices
│   ├── .bashrc
│   └── .config/nvim/
├── profiles/
│   ├── work/                  # Work-machine specific entries
│   │   └── .config/claude/
│   └── home/                  # Home-machine specific entries
│       └── .config/claude/
└── README.md
```

## Project structure

```
DotFileCommander/
├── cmd/dfc/main.go            # Entry point
├── install.sh                 # Build & install script
├── internal/
│   ├── config/config.go       # YAML config, Entry CRUD
│   ├── entry/entry.go         # Known apps, friendly names, path helpers
│   ├── hash/hash.go           # SHA256 hashing for files, dirs, symlinks
│   ├── manifest/manifest.go   # Per-entry version & hash tracking
│   ├── storage/storage.go     # Shared vs profile-specific path routing
│   ├── sync/sync.go           # Git operations, gh CLI, repo wipe
│   ├── backup/backup.go       # Copy entries to repo with progress
│   ├── restore/restore.go     # Copy from repo to filesystem
│   └── ui/
│       ├── model.go           # Root bubbletea model & view routing
│       ├── styles.go          # Lipgloss theme
│       ├── setup.go           # Setup wizard
│       ├── mainmenu.go        # Main menu
│       ├── entrylist.go       # Entry management list
│       ├── addentry.go        # Add entry flow (path → name → profile)
│       ├── configbrowser.go   # ~/.config directory browser
│       ├── backup_view.go     # Backup progress
│       ├── restore_view.go    # Restore selection + progress
│       ├── reset_view.go      # Local reset & remote wipe
│       ├── remoteview.go      # Remote sync status
│       └── profileedit.go     # Device profile management
├── go.mod
├── go.sum
└── spec.md                    # Original spec
```

## How it works

DFC uses a Git repository as a sync backend — git is an implementation detail hidden from the user. Entries are organized by profile:

- **Shared entries** → `repo/shared/<home-relative-path>`
- **Profile-specific entries** → `repo/profiles/<profile>/<home-relative-path>`

Content hashing (SHA256) ensures that changes are detected before overwriting. If a remote file has changed since your last sync, DFC warns you before restoring.

Authentication is handled through the GitHub CLI (`gh auth setup-git`), which configures git's credential helper for HTTPS.

## License

MIT

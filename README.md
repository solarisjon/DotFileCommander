# DFC — Dot File Commander

A TUI application for backing up and restoring your dotfiles to a GitHub repository. Keep your configurations in sync across multiple machines with tag-based filtering and version tracking.

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux-lightgrey)

## Features

- **Backup & Restore** — Copy your dotfile configs to/from a GitHub repo with progress bars
- **Browse ~/.config** — File browser to quickly select config directories to track
- **Tag-based filtering** — Tag entries (e.g. `home`, `work`, `laptop`) and restore only what you need
- **Version tracking** — See which entries are outdated across machines with per-entry versioning
- **GitHub CLI integration** — Uses `gh` for authentication, no manual SSH key setup required
- **TUI interface** — Built with [Charm](https://charm.sh) libraries (bubbletea, bubbles, lipgloss)

## Installation

### Prerequisites

- **Go 1.25+**
- **GitHub CLI** (`gh`) — [Install from cli.github.com](https://cli.github.com)

### Build from source

```bash
cd DotFileSync
go build -o dfc ./cmd/dfc/
```

Optionally move the binary to your PATH:

```bash
mv dfc /usr/local/bin/
```

### GitHub CLI setup

DFC uses the GitHub CLI for authentication. If you haven't already:

```bash
gh auth login
```

DFC will detect `gh` and guide you through setup on first run.

## Usage

```bash
./dfc
```

### First run

On first launch, DFC walks you through setup:

1. **GitHub CLI check** — Verifies `gh` is installed and authenticated
2. **Repository setup** — Enter an existing repo URL or create a new one via `gh`
3. **Ready** — You're taken to the main menu

### Main menu

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate |
| `Enter` | Select |
| `q` | Quit |

**Options:**

- **⬆ Backup** — Back up all tracked entries to the repo
- **⬇ Restore** — Restore entries with tag filtering and version comparison
- **📋 Manage Entries** — Add, remove, and tag tracked dotfiles
- **⚙ Settings** — Re-run setup wizard

### Managing entries

| Key | Action |
|-----|--------|
| `a` | Add a new entry manually (path → name → tags) |
| `b` | Browse `~/.config` directories to bulk-add |
| `d` | Delete selected entry |
| `t` | Edit tags on selected entry |
| `Esc` | Back to main menu |

#### Browsing ~/.config

Press `b` from the entry list to open the config browser:

1. Enter tags to apply to all selections (comma-separated, or leave blank)
2. Select directories with `Space`, `a` for all, `n` for none
3. Press `Enter` to add selected entries

Already-tracked entries are shown as dimmed with a green checkmark.

### Backup

Select **Backup** from the main menu. DFC will:

1. Sync the local repo clone
2. Copy each tracked entry into the repo (skipping `.git` subdirectories)
3. Bump version numbers in the manifest
4. Commit and push

### Restore

Select **Restore** from the main menu for a guided flow:

1. **Filter by tags** — Select `All` or pick specific tags
2. **Select entries** — Check which entries to restore, with version indicators:
   - `⬆ v1→v3` (amber) — repo has a newer version
   - `v3 ✓` (green) — up to date
3. **Progress** — Files are restored with progress bars

## Configuration

Config is stored at `~/.config/dfc/config.yaml`:

```yaml
repo_url: https://github.com/user/dotfiles.git
repo_path: /Users/you/.config/dfc/repo
entries:
  - path: ~/.config/kitty
    name: Kitty Terminal
    tags: [home, work]
    is_dir: true
    local_version: 3
  - path: ~/.config/nvim
    name: Neovim
    tags: [home]
    is_dir: true
    local_version: 2
```

### Version manifest

A `.dfc-manifest.yaml` file is stored in the git repo tracking per-entry versions:

```yaml
entries:
  ~/.config/kitty:
    version: 3
    updated_at: 2026-02-17T02:30:00Z
    updated_by: work-laptop
```

This lets DFC show which entries are outdated when you switch machines.

## Project structure

```
DotFileSync/
├── cmd/dfc/main.go           # Entry point
├── internal/
│   ├── config/config.go      # YAML config, Entry CRUD
│   ├── entry/entry.go        # Known apps, friendly names, path helpers
│   ├── manifest/manifest.go  # Per-entry version tracking
│   ├── sync/sync.go          # Git operations, gh CLI integration
│   ├── backup/backup.go      # Copy entries to repo with progress
│   ├── restore/restore.go    # Copy from repo to filesystem
│   └── ui/
│       ├── model.go          # Root bubbletea model
│       ├── styles.go         # Lipgloss theme (purple/cyan/green)
│       ├── setup.go          # Setup wizard
│       ├── mainmenu.go       # Main menu
│       ├── entrylist.go      # Entry management list
│       ├── addentry.go       # Add entry flow
│       ├── tagedit.go        # Tag editor
│       ├── configbrowser.go  # ~/.config directory browser
│       ├── backup_view.go    # Backup progress
│       └── restore_view.go   # Restore selection + progress
├── go.mod
├── go.sum
└── spec.md                   # Original spec
```

## How it works

DFC uses a git repository as a sync backend — but git is an implementation detail hidden from the user. Files are organized in the repo mirroring their home-relative paths:

```
repo/
├── .config/
│   ├── kitty/
│   │   └── kitty.conf
│   ├── nvim/
│   │   ├── init.lua
│   │   └── lua/...
│   └── starship.toml
├── .dfc-manifest.yaml
└── README.md
```

Authentication is handled entirely through the GitHub CLI (`gh auth setup-git`), which configures git's credential helper for HTTPS.

## License

Internal tool — NetApp/CPET.

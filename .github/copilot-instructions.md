# Copilot Instructions — DotFileSync

## Project Overview

A terminal-based dotfile backup and restore tool. Users can sync dotfiles and `.config` entries across multiple computers, with tagging support (e.g., `home`, `work`) to control which files restore where. See `spec.md` for full requirements.

## Architecture

- **Language**: Go 1.25+, **TUI**: Charm ecosystem (bubbletea + bubbles + lipgloss)
- **Layout**: `cmd/dfc/main.go` entry point, `internal/` for all packages
- **Config**: `~/.config/dfc/config.yaml` (YAML) — tracked entries, repo URL, tags
- **Backup storage**: Git repo (user-provided URL or created via `gh` CLI)

### Packages

| Package | Purpose |
|---------|---------|
| `internal/config` | Load/save YAML config, Entry model (path, name, tags, is_dir) |
| `internal/entry` | Friendly name mapping for known apps, `.config` directory listing |
| `internal/sync` | Git operations: clone, pull, commit+push, `gh repo create` |
| `internal/backup` | Copy tracked entries into repo working tree, progress channel |
| `internal/restore` | Copy from repo to filesystem with tag filtering, progress channel |
| `internal/ui` | All bubbletea views: setup, main menu, entry list, add/tag edit, backup/restore progress |

## Build & Test Commands

```bash
go build -o dfc ./cmd/dfc
go test ./internal/...                              # all tests
go test ./internal/config/...                       # single package
go test -run TestFunctionName ./internal/config/...  # single test
```

## Key Requirements from Spec

- Backup and restore dotfiles across multiple machines
- Tag-based filtering (e.g., `home`, `work`) controls which entries restore on which machine
- Support both single files (e.g., `~/.wezterm`) and directories (e.g., `~/.config/kitty`)
- Display user-friendly names for entries (e.g., `.config/kitty` → "Kitty Term")
- Progress bars per entry during backup/restore operations
- Directory browsing UI when adding `.config` entries

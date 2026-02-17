package entry

import (
	"os"
	"path/filepath"
	"strings"
)

// KnownApps maps common .config directory names to friendly descriptions.
var KnownApps = map[string]string{
	"kitty":       "Kitty Terminal",
	"wezterm":     "WezTerm Terminal",
	"alacritty":   "Alacritty Terminal",
	"nvim":        "Neovim",
	"fish":        "Fish Shell",
	"zsh":         "Zsh Shell",
	"starship":    "Starship Prompt",
	"tmux":        "Tmux Multiplexer",
	"ghostty":     "Ghostty Terminal",
	"helix":       "Helix Editor",
	"lazygit":     "LazyGit",
	"bat":         "Bat (cat replacement)",
	"btop":        "Btop System Monitor",
	"htop":        "Htop Process Viewer",
	"scs-connect": "SCS Lab Connector",
	"ssc":         "SSH Commander",
	"karabiner":   "Karabiner-Elements",
	"yabai":       "Yabai Window Manager",
	"skhd":        "skhd Hotkey Daemon",
	"aerospace":   "AeroSpace Window Manager",
	"borders":     "JankyBorders",
	"raycast":     "Raycast",
}

// FriendlyName returns a human-readable name for a path.
// For .config subdirectories, it checks KnownApps.
// For standalone dotfiles, it strips the leading dot.
func FriendlyName(path string) string {
	path = expandHome(path)
	base := filepath.Base(path)

	// Check if it's a .config subdirectory
	parent := filepath.Dir(path)
	if filepath.Base(parent) == ".config" {
		if name, ok := KnownApps[base]; ok {
			return name
		}
		// Capitalize the directory name
		return strings.Title(base)
	}

	// Standalone dotfiles — strip leading dot
	name := strings.TrimPrefix(base, ".")
	return strings.Title(name)
}

// ListConfigDirs returns subdirectories of ~/.config for browsing.
func ListConfigDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(home, ".config")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

// IsDir checks whether a path is a directory.
func IsDir(path string) bool {
	path = expandHome(path)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Exists checks whether a path exists.
func Exists(path string) bool {
	path = expandHome(path)
	_, err := os.Stat(path)
	return err == nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

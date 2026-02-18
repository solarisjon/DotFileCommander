package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
)

// RepoDir computes the destination directory inside the repo for an entry.
// Shared entries:  repo/shared/<homeRelPath>
// Profile entries: repo/profiles/<profile>/<homeRelPath>
func RepoDir(entry config.Entry, profile string) string {
	rel := homeRelative(entry.Path)
	if entry.ProfileSpecific && profile != "" {
		return filepath.Join("profiles", strings.ToLower(profile), rel)
	}
	return filepath.Join("shared", rel)
}

// ManifestKey returns the manifest map key for an entry.
// Format: "shared/<path>" or "profiles/<profile>/<path>"
func ManifestKey(entry config.Entry, profile string) string {
	if entry.ProfileSpecific && profile != "" {
		return "profiles/" + strings.ToLower(profile) + "/" + entry.Path
	}
	return "shared/" + entry.Path
}

// LegacyRepoDir returns the old-style repo path (directly under repo root).
func LegacyRepoDir(entry config.Entry) string {
	return homeRelative(entry.Path)
}

// LegacyManifestKey returns the old-style manifest key (raw entry path).
func LegacyManifestKey(entry config.Entry) string {
	return entry.Path
}

func homeRelative(path string) string {
	path = expandHome(path)
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Base(path)
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/entry"
	"github.com/solarisjon/dfc/internal/manifest"
)

// RepoEntry is an entry discovered from the repo manifest.
type RepoEntry struct {
	Entry   config.Entry
	Version int
}

// ListRepoEntries reads the manifest and returns all entries stored in the repo.
// Entries already tracked in existing are excluded.
// For profile-specific entries, only those matching currentProfile are returned (all if empty).
func ListRepoEntries(repoPath, currentProfile string, existing []config.Entry) ([]RepoEntry, error) {
	repoPath = expandHome(repoPath)
	m, err := manifest.Load(repoPath)
	if err != nil {
		return nil, err
	}

	existingPaths := make(map[string]bool)
	for _, e := range existing {
		existingPaths[e.Path] = true
	}

	var result []RepoEntry
	for key, ev := range m.Entries {
		var entryPath string
		var profileSpecific bool
		var profileName string

		if strings.HasPrefix(key, "shared/") {
			rest := strings.TrimPrefix(key, "shared/")
			if !strings.HasPrefix(rest, "~/") {
				rest = "~/" + rest
			}
			entryPath = rest
		} else if strings.HasPrefix(key, "profiles/") {
			without := strings.TrimPrefix(key, "profiles/")
			slash := strings.Index(without, "/")
			if slash < 0 {
				continue
			}
			profileName = without[:slash]
			rest := without[slash+1:]
			if currentProfile != "" && !strings.EqualFold(profileName, currentProfile) {
				continue
			}
			if !strings.HasPrefix(rest, "~/") {
				rest = "~/" + rest
			}
			entryPath = rest
			profileSpecific = true
		} else {
			continue
		}

		if existingPaths[entryPath] {
			continue
		}

		// Stat the actual repo path to determine if it's a directory
		cleanPath := entryPath
		if strings.HasPrefix(cleanPath, "~/") {
			cleanPath = cleanPath[2:]
		}
		var repoRelPath string
		if profileSpecific {
			repoRelPath = filepath.Join("profiles", profileName, cleanPath)
		} else {
			repoRelPath = filepath.Join("shared", cleanPath)
		}
		fullPath := filepath.Join(repoPath, repoRelPath)
		info, statErr := os.Stat(fullPath)
		isDir := statErr == nil && info.IsDir()

		result = append(result, RepoEntry{
			Entry: config.Entry{
				Path:            entryPath,
				Name:            entry.FriendlyName(entryPath),
				IsDir:           isDir,
				ProfileSpecific: profileSpecific,
			},
			Version: ev.Version,
		})
	}
	return result, nil
}

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

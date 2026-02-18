package storage

import (
	"os"
	"path/filepath"

	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/manifest"
)

// MigrateLegacyLayout moves entries from the old flat repo layout into
// shared/ (or profiles/<p>/ for profile-specific entries). Also migrates
// manifest keys. Returns the number of entries migrated.
func MigrateLegacyLayout(cfg *config.Config, mf *manifest.Manifest) (int, error) {
	repoPath := expandHome(cfg.RepoPath)
	migrated := 0

	for _, entry := range cfg.Entries {
		legacyRel := LegacyRepoDir(entry)
		legacyFull := filepath.Join(repoPath, legacyRel)

		// Check if legacy path exists and new path doesn't
		newRel := RepoDir(entry, cfg.DeviceProfile)
		newFull := filepath.Join(repoPath, newRel)

		if _, err := os.Stat(legacyFull); os.IsNotExist(err) {
			continue // nothing to migrate
		}
		if _, err := os.Stat(newFull); err == nil {
			continue // already migrated
		}

		// Create parent dirs and rename
		if err := os.MkdirAll(filepath.Dir(newFull), 0755); err != nil {
			return migrated, err
		}
		if err := os.Rename(legacyFull, newFull); err != nil {
			return migrated, err
		}

		// Migrate manifest key
		legacyKey := LegacyManifestKey(entry)
		newKey := ManifestKey(entry, cfg.DeviceProfile)
		if ev, ok := mf.Entries[legacyKey]; ok {
			mf.Entries[newKey] = ev
			delete(mf.Entries, legacyKey)
		}

		migrated++
	}

	return migrated, nil
}

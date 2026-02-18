package restore

import (
	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/hash"
	"github.com/solarisjon/dfc/internal/manifest"
)

// ConflictState describes the sync state of a local entry vs the repo.
type ConflictState int

const (
	StateClean         ConflictState = iota // local matches last known hash
	StateNewerInRepo                        // repo has newer version, local unchanged
	StateModifiedLocal                      // local changed since last backup/restore
	StateConflict                           // both local and repo changed
	StateUnknown                            // no prior hash to compare (first time)
)

func (s ConflictState) String() string {
	switch s {
	case StateClean:
		return "clean"
	case StateNewerInRepo:
		return "newer in repo"
	case StateModifiedLocal:
		return "modified locally"
	case StateConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// ConflictResult holds the conflict analysis for a single entry.
type ConflictResult struct {
	Entry     config.Entry
	State     ConflictState
	LocalHash string // current hash of local file/dir
	RepoHash  string // hash stored in manifest
}

// CheckConflicts computes the conflict state for each entry by comparing the
// current local content hash against the last-known hash (stored in config)
// and the repo manifest version/hash.
func CheckConflicts(entries []config.Entry, mf *manifest.Manifest) []ConflictResult {
	results := make([]ConflictResult, len(entries))

	for i, e := range entries {
		cr := ConflictResult{Entry: e}

		// Get repo info
		mv := mf.GetEntry(e.Path)
		cr.RepoHash = mv.ContentHash

		// Compute current local hash
		localHash, err := hash.HashEntry(e)
		if err != nil {
			// Can't hash local (file missing, etc) — safe to restore
			cr.State = StateClean
			results[i] = cr
			continue
		}
		cr.LocalHash = localHash

		repoNewer := mv.Version > e.LocalVersion

		// No prior hash recorded — if local file exists and differs from
		// what's in the repo, treat as a conflict to avoid silent overwrite.
		if e.LastHash == "" {
			if cr.RepoHash != "" && localHash != cr.RepoHash {
				// Local file differs from repo content — warn user
				if repoNewer {
					cr.State = StateConflict
				} else {
					cr.State = StateModifiedLocal
				}
			} else {
				cr.State = StateClean
			}
			results[i] = cr
			continue
		}

		localModified := localHash != e.LastHash

		switch {
		case !localModified && !repoNewer:
			cr.State = StateClean
		case !localModified && repoNewer:
			// Repo is newer but local unchanged — still check if the actual
			// content differs so user knows their file will change.
			if cr.RepoHash != "" && localHash != cr.RepoHash {
				cr.State = StateNewerInRepo
			} else {
				// Repo version bumped but content is identical — safe
				cr.State = StateClean
			}
		case localModified && !repoNewer:
			cr.State = StateModifiedLocal
		case localModified && repoNewer:
			cr.State = StateConflict
		}

		results[i] = cr
	}

	return results
}

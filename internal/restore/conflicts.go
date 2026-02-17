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
			// Can't hash local (file missing, etc) — treat as unknown
			cr.State = StateUnknown
			results[i] = cr
			continue
		}
		cr.LocalHash = localHash

		// No prior hash recorded — first time, can't detect changes
		if e.LastHash == "" {
			cr.State = StateUnknown
			results[i] = cr
			continue
		}

		localModified := localHash != e.LastHash
		repoNewer := mv.Version > e.LocalVersion

		switch {
		case !localModified && !repoNewer:
			cr.State = StateClean
		case !localModified && repoNewer:
			cr.State = StateNewerInRepo
		case localModified && !repoNewer:
			cr.State = StateModifiedLocal
		case localModified && repoNewer:
			cr.State = StateConflict
		}

		results[i] = cr
	}

	return results
}

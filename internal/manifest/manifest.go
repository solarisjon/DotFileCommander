package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EntryVersion tracks the version of a single backed-up entry.
type EntryVersion struct {
	Version     int       `yaml:"version"`
	UpdatedAt   time.Time `yaml:"updated_at"`
	UpdatedBy   string    `yaml:"updated_by,omitempty"` // hostname
	ContentHash string    `yaml:"content_hash,omitempty"`
}

// Manifest tracks versions of all entries in the repo.
// Stored as .dfc-manifest.yaml in the repo root.
type Manifest struct {
	Entries map[string]EntryVersion `yaml:"entries"` // keyed by entry path
}

const fileName = ".dfc-manifest.yaml"

// Load reads the manifest from the repo. Returns empty manifest if not found.
func Load(repoPath string) (*Manifest, error) {
	repoPath = expandHome(repoPath)
	path := filepath.Join(repoPath, fileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Entries: make(map[string]EntryVersion)}, nil
		}
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if m.Entries == nil {
		m.Entries = make(map[string]EntryVersion)
	}
	return &m, nil
}

// Save writes the manifest to the repo.
func (m *Manifest) Save(repoPath string) error {
	repoPath = expandHome(repoPath)
	path := filepath.Join(repoPath, fileName)

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// BumpVersion increments the version for an entry path and records the timestamp and hash.
func (m *Manifest) BumpVersion(entryPath string, contentHash string) {
	ev := m.Entries[entryPath]
	ev.Version++
	ev.UpdatedAt = time.Now()
	ev.ContentHash = contentHash
	if host, err := os.Hostname(); err == nil {
		ev.UpdatedBy = host
	}
	m.Entries[entryPath] = ev
}

// GetVersion returns the repo version for an entry path (0 if never backed up).
func (m *Manifest) GetVersion(entryPath string) int {
	return m.Entries[entryPath].Version
}

// GetEntry returns the full version info for an entry path.
func (m *Manifest) GetEntry(entryPath string) EntryVersion {
	return m.Entries[entryPath]
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

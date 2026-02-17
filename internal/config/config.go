package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Entry represents a tracked dotfile or directory.
type Entry struct {
	Path         string   `yaml:"path"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	IsDir        bool     `yaml:"is_dir,omitempty"`
	LocalVersion int      `yaml:"local_version,omitempty"` // last backed-up or restored version
}

// Config holds all dfc configuration.
type Config struct {
	RepoURL  string  `yaml:"repo_url"`
	RepoPath string  `yaml:"repo_path"`
	Entries  []Entry `yaml:"entries,omitempty"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "dfc"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// DefaultRepoPath returns the default local clone location.
func DefaultRepoPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repo"), nil
}

// Load reads config from disk. Returns empty config if file doesn't exist.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			repoPath, _ := DefaultRepoPath()
			return &Config{RepoPath: repoPath}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.RepoPath == "" {
		cfg.RepoPath, _ = DefaultRepoPath()
	}

	return &cfg, nil
}

// Save writes config to disk.
func (cfg *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path, err := Path()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// IsConfigured returns true if a repo URL has been set.
func (cfg *Config) IsConfigured() bool {
	return cfg.RepoURL != ""
}

// AddEntry adds a new tracked entry and saves.
func (cfg *Config) AddEntry(e Entry) error {
	cfg.Entries = append(cfg.Entries, e)
	return cfg.Save()
}

// RemoveEntry removes an entry by index and saves.
func (cfg *Config) RemoveEntry(index int) error {
	if index < 0 || index >= len(cfg.Entries) {
		return fmt.Errorf("index %d out of range", index)
	}
	cfg.Entries = append(cfg.Entries[:index], cfg.Entries[index+1:]...)
	return cfg.Save()
}

// UpdateEntry replaces an entry at the given index and saves.
func (cfg *Config) UpdateEntry(index int, e Entry) error {
	if index < 0 || index >= len(cfg.Entries) {
		return fmt.Errorf("index %d out of range", index)
	}
	cfg.Entries[index] = e
	return cfg.Save()
}

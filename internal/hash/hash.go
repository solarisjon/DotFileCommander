package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
)

// HashFile returns the hex-encoded SHA256 of a single file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hash file %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashDir returns a deterministic SHA256 for a directory tree.
// It walks files in sorted order, hashing each file's relative path
// and content into a single digest. Skips .git directories.
func HashDir(path string) (string, error) {
	// Collect all file paths first, then sort for determinism.
	var files []string
	var symlinks []string
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type()&fs.ModeSymlink != 0 {
			symlinks = append(symlinks, p)
			return nil
		}
		// Skip special files (sockets, pipes, devices)
		if !d.IsDir() && !d.Type().IsRegular() {
			return nil
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash dir %s: %w", path, err)
	}

	sort.Strings(files)
	sort.Strings(symlinks)

	h := sha256.New()
	for _, fp := range files {
		rel, err := filepath.Rel(path, fp)
		if err != nil {
			continue // skip files we can't resolve
		}
		// Include the relative path in the hash so renames are detected.
		h.Write([]byte(rel))

		f, err := os.Open(fp)
		if err != nil {
			continue // skip unreadable files
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			continue // skip files that fail mid-read
		}
		f.Close()
	}

	// Include symlinks: hash their relative path + link target
	for _, sp := range symlinks {
		rel, err := filepath.Rel(path, sp)
		if err != nil {
			continue
		}
		h.Write([]byte("symlink:" + rel))
		linkTarget, err := os.Readlink(sp)
		if err != nil {
			continue // skip broken symlinks
		}
		h.Write([]byte(linkTarget))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashEntry hashes the local file or directory for a config entry.
func HashEntry(e config.Entry) (string, error) {
	path := expandHome(e.Path)
	if e.IsDir {
		return HashDir(path)
	}
	return HashFile(path)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

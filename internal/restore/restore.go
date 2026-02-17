package restore

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
)

// Progress reports the status of a single entry restore.
type Progress struct {
	Entry       config.Entry
	Index       int
	Total       int
	Done        bool
	Err         error
	BytesCopied int64
	BytesTotal  int64
}

// FilterByTags returns entries that have at least one matching tag.
// If tags is empty, all entries are returned.
func FilterByTags(entries []config.Entry, tags []string) []config.Entry {
	if len(tags) == 0 {
		return entries
	}
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var filtered []config.Entry
	for _, e := range entries {
		for _, t := range e.Tags {
			if tagSet[strings.ToLower(t)] {
				filtered = append(filtered, e)
				break
			}
		}
	}
	return filtered
}

// Run restores entries from the repo to the filesystem.
func Run(entries []config.Entry, repoPath string) <-chan Progress {
	ch := make(chan Progress)

	go func() {
		defer close(ch)

		repoPath = expandHome(repoPath)
		total := len(entries)

		for i, entry := range entries {
			p := Progress{Entry: entry, Index: i, Total: total}

			relPath := homeRelative(entry.Path)
			srcPath := filepath.Join(repoPath, relPath)
			dstPath := expandHome(entry.Path)

			var err error
			if entry.IsDir {
				err = copyDir(srcPath, dstPath, &p)
			} else {
				err = copyFile(srcPath, dstPath, &p)
			}

			p.Done = true
			p.Err = err
			ch <- p
		}
	}()

	return ch
}

func copyFile(src, dst string, p *Progress) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	p.BytesTotal = info.Size()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, in)
	p.BytesCopied = n
	if err != nil {
		return err
	}

	return out.Chmod(info.Mode())
}

func copyDir(src, dst string, p *Progress) error {
	var totalBytes int64
	_ = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return err
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		totalBytes += info.Size()
		return nil
	})
	p.BytesTotal = totalBytes

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		n, err := io.Copy(out, in)
		p.BytesCopied += n
		if err != nil {
			return err
		}

		return out.Chmod(info.Mode())
	})
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

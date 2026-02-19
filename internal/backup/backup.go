package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/hash"
	"github.com/solarisjon/dfc/internal/storage"
)

// Progress reports the status of a single entry backup.
type Progress struct {
	Entry       config.Entry
	Index       int
	Total       int
	Done        bool
	Err         error
	BytesCopied int64
	BytesTotal  int64
	ContentHash string // SHA256 hash of the source after backup
	Skipped     int    // number of files skipped due to errors
	Copied      int    // number of files successfully copied
	Warning     string // human-readable warning if something noteworthy happened
}

// Run backs up all entries into the repo working tree.
// It sends progress updates on the returned channel.
// The profile parameter determines where profile-specific entries are stored.
func Run(entries []config.Entry, repoPath string, profile string) <-chan Progress {
	ch := make(chan Progress)

	go func() {
		defer close(ch)

		repoPath = expandHome(repoPath)
		total := len(entries)

		for i, entry := range entries {
			p := Progress{Entry: entry, Index: i, Total: total}

			srcPath := expandHome(entry.Path)
			// Use storage paths: shared/ or profiles/<profile>/
			relPath := storage.RepoDir(entry, profile)
			destPath := filepath.Join(repoPath, relPath)

			var err error
			if entry.IsDir {
				err = copyDir(srcPath, destPath, &p)
			} else {
				err = copyFile(srcPath, destPath, &p)
			}

			p.Done = true
			p.Err = err
			if err == nil {
				// Generate warnings for entries with nothing useful to back up
				if entry.IsDir && p.Copied == 0 && p.Skipped > 0 {
					p.Warning = describeSkippedDir(srcPath)
				}
				// Compute hash of the source for state tracking
				h, hashErr := hash.HashEntry(entry)
				if hashErr == nil {
					p.ContentHash = h
				}
			}
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
	// Count total bytes first (skip .git dirs and symlinks)
	var totalBytes int64
	_ = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // skip symlinks for byte counting
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
			// Skip files/dirs we can't access rather than aborting
			p.Skipped++
			return nil
		}

		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			p.Skipped++
			return nil
		}
		target := filepath.Join(dst, rel)

		// Handle symlinks: recreate them rather than following
		if d.Type()&fs.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				p.Skipped++
				return nil
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				p.Skipped++
				return nil
			}
			// Remove existing symlink/file at target before creating
			os.Remove(target)
			if err := os.Symlink(linkTarget, target); err != nil {
				p.Skipped++
			}
			return nil
		}

		// Skip special files (sockets, pipes, devices)
		if !d.IsDir() && d.Type().IsRegular() == false {
			p.Skipped++
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		info, err := d.Info()
		if err != nil {
			p.Skipped++
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			p.Skipped++
			return nil
		}

		in, err := os.Open(path)
		if err != nil {
			p.Skipped++
			return nil
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			p.Skipped++
			return nil
		}
		defer out.Close()

		n, err := io.Copy(out, in)
		p.BytesCopied += n
		if err != nil {
			p.Skipped++
			return nil
		}

		p.Copied++
		return out.Chmod(info.Mode())
	})
}

// describeSkippedDir inspects a directory to explain why nothing was copied.
func describeSkippedDir(dir string) string {
	var symlinks, sockets, other int
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			symlinks++
		} else if d.Type()&fs.ModeSocket != 0 {
			sockets++
		} else if !d.Type().IsRegular() {
			other++
		}
		return nil
	})

	parts := []string{}
	if symlinks > 0 {
		parts = append(parts, fmt.Sprintf("%d symlink(s)", symlinks))
	}
	if sockets > 0 {
		parts = append(parts, fmt.Sprintf("%d socket(s)", sockets))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("%d special file(s)", other))
	}

	if len(parts) == 0 {
		return "no regular files found"
	}
	return "only contains " + strings.Join(parts, ", ") + " — nothing to back up"
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

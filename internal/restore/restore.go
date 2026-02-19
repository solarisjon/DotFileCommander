package restore

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/solarisjon/dfc/internal/config"
	"github.com/solarisjon/dfc/internal/storage"
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
	Skipped     int      // number of files skipped due to errors
	SkipReasons []string // why each file was skipped
}

// Run restores entries from the repo to the filesystem.
// The profile parameter determines where profile-specific entries are read from.
func Run(entries []config.Entry, repoPath string, profile string) <-chan Progress {
	ch := make(chan Progress)

	go func() {
		defer close(ch)

		repoPath = expandHome(repoPath)
		total := len(entries)

		for i, entry := range entries {
			p := Progress{Entry: entry, Index: i, Total: total}

			// Use storage paths: shared/ or profiles/<profile>/
			relPath := storage.RepoDir(entry, profile)
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
			skipFile(p, path, src, fmt.Sprintf("access error: %v", err))
			return nil
		}

		// Skip .git directories
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			skipFile(p, path, src, fmt.Sprintf("path error: %v", err))
			return nil
		}
		target := filepath.Join(dst, rel)

		// Handle symlinks: recreate them rather than following
		if d.Type()&fs.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				skipFile(p, path, src, fmt.Sprintf("symlink read error: %v", err))
				return nil
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				skipFile(p, path, src, fmt.Sprintf("mkdir error: %v", err))
				return nil
			}
			os.Remove(target)
			if err := os.Symlink(linkTarget, target); err != nil {
				skipFile(p, path, src, fmt.Sprintf("symlink create error: %v", err))
			}
			return nil
		}

		// Skip special files (sockets, pipes, devices)
		if !d.IsDir() && !d.Type().IsRegular() {
			skipFile(p, path, src, "special file (socket/pipe/device)")
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		info, err := d.Info()
		if err != nil {
			skipFile(p, path, src, fmt.Sprintf("stat error: %v", err))
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			skipFile(p, path, src, fmt.Sprintf("mkdir error: %v", err))
			return nil
		}

		in, err := os.Open(path)
		if err != nil {
			skipFile(p, path, src, fmt.Sprintf("open error: %v", err))
			return nil
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			skipFile(p, path, src, fmt.Sprintf("create error: %v", err))
			return nil
		}
		defer out.Close()

		n, err := io.Copy(out, in)
		p.BytesCopied += n
		if err != nil {
			skipFile(p, path, src, fmt.Sprintf("copy error: %v", err))
			return nil
		}

		return out.Chmod(info.Mode())
	})
}

func skipFile(p *Progress, path, base, reason string) {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	p.Skipped++
	p.SkipReasons = append(p.SkipReasons, rel+": "+reason)
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

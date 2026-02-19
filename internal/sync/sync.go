package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GhStatus describes the state of the GitHub CLI.
type GhStatus int

const (
	GhChecking         GhStatus = iota
	GhNotInstalled
	GhNotAuthenticated
	GhReady
)

// CheckGh checks whether the gh CLI is installed and authenticated.
func CheckGh() GhStatus {
	ghPath := findGh()
	if ghPath == "" {
		return GhNotInstalled
	}
	cmd := exec.Command(ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		return GhNotAuthenticated
	}
	return GhReady
}

// findGh locates the gh binary, checking PATH and common install locations.
func findGh() string {
	if p, err := exec.LookPath("gh"); err == nil {
		return p
	}
	// Check common locations not always in PATH
	for _, p := range []string{
		"/opt/homebrew/bin/gh",
		"/usr/local/bin/gh",
		"/usr/bin/gh",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ghBin returns the path to the gh binary (cached after first lookup).
var ghBin string

func getGhBin() string {
	if ghBin == "" {
		ghBin = findGh()
	}
	return ghBin
}

// RunGhAuth launches `gh auth login` interactively.
func RunGhAuth() error {
	cmd := exec.Command(getGhBin(), "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SetupGitCredentialHelper configures git to use gh as the credential helper
// so HTTPS clones/pushes authenticate automatically.
func SetupGitCredentialHelper() error {
	return exec.Command(getGhBin(), "auth", "setup-git").Run()
}

// EnsureRepo clones the repo if it doesn't exist locally, or pulls latest.
func EnsureRepo(repoURL, localPath string) error {
	localPath = expandHome(localPath)

	if _, err := os.Stat(filepath.Join(localPath, ".git")); os.IsNotExist(err) {
		// Remove empty directory if it exists (left from a failed clone)
		os.RemoveAll(localPath)
		return clone(repoURL, localPath)
	}

	// Verify the existing clone points to the correct remote URL.
	// If the user changed their repo URL in settings, re-clone.
	currentURL, _ := gitOutput(localPath, "remote", "get-url", "origin")
	currentURL = strings.TrimSpace(currentURL)
	if currentURL != "" && currentURL != repoURL {
		os.RemoveAll(localPath)
		return clone(repoURL, localPath)
	}

	return pull(localPath)
}

// CommitAndPush stages all changes, commits, and pushes.
func CommitAndPush(localPath, message string) error {
	localPath = expandHome(localPath)

	if err := gitCmd(localPath, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there's anything to commit
	out, err := gitOutput(localPath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return nil // nothing to commit
	}

	if err := gitCmd(localPath, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := gitCmd(localPath, "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

// CreateGitHubRepo creates a new private GitHub repo via the gh CLI
// and returns the HTTPS clone URL.
func CreateGitHubRepo(name string) (string, error) {
	cmd := exec.Command(getGhBin(), "repo", "create", name, "--private", "--clone=false")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh repo create: %s: %w", string(out), err)
	}

	// Parse the URL from gh output
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") || strings.HasPrefix(line, "git@") {
			return line, nil
		}
	}

	// Fallback: construct HTTPS URL from name
	if !strings.Contains(name, "/") {
		// Get current gh user
		user, err := gitOutput("", "config", "user.name")
		if err == nil && strings.TrimSpace(user) != "" {
			name = strings.TrimSpace(user) + "/" + name
		}
	}
	return fmt.Sprintf("https://github.com/%s.git", name), nil
}

// InitRepo initializes a new git repo at the given path with an initial commit.
func InitRepo(localPath string) error {
	localPath = expandHome(localPath)
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return err
	}
	if err := gitCmd(localPath, "init"); err != nil {
		return err
	}
	// Create a README so we have something to commit
	readme := filepath.Join(localPath, "README.md")
	if err := os.WriteFile(readme, []byte("# Dotfiles\n\nManaged by [dfc](https://github.com/solarisjon/dfc) (Dot File Commander).\n"), 0644); err != nil {
		return err
	}
	if err := gitCmd(localPath, "add", "-A"); err != nil {
		return err
	}
	if err := gitCmd(localPath, "commit", "-m", "Initial commit from dfc"); err != nil {
		return err
	}
	return gitCmd(localPath, "branch", "-M", "main")
}

// AddRemoteAndPush adds a remote and pushes the initial commit.
func AddRemoteAndPush(localPath, url string) error {
	localPath = expandHome(localPath)
	if err := gitCmd(localPath, "remote", "add", "origin", url); err != nil {
		return fmt.Errorf("adding remote: %w", err)
	}
	if err := gitCmd(localPath, "push", "-u", "origin", "main"); err != nil {
		return fmt.Errorf("initial push: %w", err)
	}
	return nil
}

func clone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	// Use a known-good CWD so clone works even if the process CWD was deleted
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %s: %w", string(out), err)
	}
	// If the repo is empty, create an initial commit
	gitDir := filepath.Join(dest, ".git")
	if _, statErr := os.Stat(gitDir); statErr == nil {
		// Check if HEAD exists (empty repo won't have one)
		headCmd := exec.Command("git", "rev-parse", "HEAD")
		headCmd.Dir = dest
		if headErr := headCmd.Run(); headErr != nil {
			// Empty repo — seed it
			readme := filepath.Join(dest, "README.md")
			_ = os.WriteFile(readme, []byte("# Dotfiles\n\nManaged by dfc (Dot File Commander).\n"), 0644)
			_ = gitCmd(dest, "add", "-A")
			_ = gitCmd(dest, "commit", "-m", "Initial commit from dfc")
			_ = gitCmd(dest, "branch", "-M", "main")
			_ = gitCmd(dest, "push", "-u", "origin", "main")
		}
	}
	return nil
}

func pull(dir string) error {
	// Only pull if there's a remote and commits exist
	out, err := gitOutput(dir, "remote")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	// Check if there are any commits
	if err := gitCmd(dir, "rev-parse", "HEAD"); err != nil {
		return nil // no commits yet, nothing to pull
	}
	// Check if upstream is configured
	_, err = gitOutput(dir, "rev-parse", "--abbrev-ref", "@{u}")
	if err != nil {
		return nil // no upstream tracking branch
	}
	return gitCmd(dir, "pull", "--ff-only")
}

func gitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s: %w", args[0], string(out), err)
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	return string(out), err
}

// NukeRepo completely resets the remote repo by removing all content,
// creating a fresh initial commit, and force-pushing it. This destroys
// all remote history and data.
func NukeRepo(localPath string) error {
	localPath = expandHome(localPath)

	// Remove everything except .git
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("reading repo dir: %w", err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(localPath, e.Name())); err != nil {
			return fmt.Errorf("removing %s: %w", e.Name(), err)
		}
	}

	// Create a fresh README
	readme := filepath.Join(localPath, "README.md")
	if err := os.WriteFile(readme, []byte("# Dotfiles\n\nManaged by [dfc](https://github.com/solarisjon/DotFileCommander) (Dot File Commander).\n"), 0644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}

	// Stage, commit, and force push
	if err := gitCmd(localPath, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitCmd(localPath, "commit", "-m", "Reset repo — wiped by dfc"); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	// Force push to overwrite remote history
	if err := gitCmd(localPath, "push", "--force"); err != nil {
		return fmt.Errorf("git push --force: %w", err)
	}
	return nil
}

// GitIdentity holds the user's git config name and email.
type GitIdentity struct {
	Name  string
	Email string
}

// CheckGitIdentity reads git's global user.name and user.email.
func CheckGitIdentity() GitIdentity {
	name, _ := gitOutput("", "config", "--global", "user.name")
	email, _ := gitOutput("", "config", "--global", "user.email")
	return GitIdentity{
		Name:  strings.TrimSpace(name),
		Email: strings.TrimSpace(email),
	}
}

// SetGitIdentity sets git's global user.name and user.email.
func SetGitIdentity(name, email string) error {
	if err := gitCmd("", "config", "--global", "user.name", name); err != nil {
		return fmt.Errorf("setting user.name: %w", err)
	}
	if err := gitCmd("", "config", "--global", "user.email", email); err != nil {
		return fmt.Errorf("setting user.email: %w", err)
	}
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

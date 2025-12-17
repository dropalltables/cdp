package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dropalltables/cdp/internal/config"
)

// IsRepo checks if the directory is a git repository
func IsRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Init initializes a new git repository
func Init(dir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	return cmd.Run()
}

// GetRemoteURL returns the remote URL for the given remote name
func GetRemoteURL(dir, remoteName string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", remoteName)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SetRemote sets or updates a remote URL
func SetRemote(dir, remoteName, url string) error {
	// Try to add first, if it fails, update
	cmd := exec.Command("git", "remote", "add", remoteName, url)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "remote", "set-url", remoteName, url)
		cmd.Dir = dir
		return cmd.Run()
	}
	return nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return config.DefaultBranch, nil
	}
	return branch, nil
}

// HasChanges checks if there are uncommitted changes
func HasChanges(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// AddAll stages all changes
func AddAll(dir string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	return cmd.Run()
}

// Commit creates a commit with the given message
func Commit(dir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	// Silence output during deployment
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// Push pushes to the remote
func Push(dir, remoteName, branch string) error {
	cmd := exec.Command("git", "push", "-u", remoteName, branch)
	cmd.Dir = dir
	// Silence output during deployment
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// GetLatestCommitHash returns the latest commit hash
func GetLatestCommitHash(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// AutoCommit stages all changes and creates a commit
func AutoCommit(dir string) error {
	if !HasChanges(dir) {
		return nil // Nothing to commit
	}

	if err := AddAll(dir); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	message := fmt.Sprintf("Deploy via cdp")
	return Commit(dir, message)
}

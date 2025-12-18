package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
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
	return CommitVerbose(dir, message, false)
}

// CommitVerbose creates a commit with optional output
func CommitVerbose(dir, message string, verbose bool) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	if verbose {
		// Stream output with dim styling like deployment logs
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			return err
		}

		done := make(chan bool, 2)

		// Print stdout
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		// Print stderr
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		<-done
		<-done

		return cmd.Wait()
	}
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

// PushWithToken pushes to the remote using token-based authentication
func PushWithToken(dir, remoteName, branch, token string) error {
	return PushWithTokenVerbose(dir, remoteName, branch, token, false)
}

// PushWithTokenVerbose pushes to the remote using token-based authentication with optional output
func PushWithTokenVerbose(dir, remoteName, branch, token string, verbose bool) error {
	// Get current remote URL
	currentURL, err := GetRemoteURL(dir, remoteName)
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	// Inject token into URL temporarily
	var urlWithToken string
	if strings.HasPrefix(currentURL, "https://github.com/") {
		urlWithToken = strings.Replace(currentURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", token), 1)
	} else {
		return fmt.Errorf("unsupported remote URL format: %s", currentURL)
	}

	// Temporarily update remote URL
	if err := SetRemote(dir, remoteName, urlWithToken); err != nil {
		return fmt.Errorf("failed to set remote URL: %w", err)
	}

	// Restore original URL after push
	defer SetRemote(dir, remoteName, currentURL)

	// Push
	cmd := exec.Command("git", "push", "-u", remoteName, branch)
	cmd.Dir = dir

	if verbose {
		// Stream output with dim styling like deployment logs
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			return err
		}

		// Combine stdout and stderr for git push (progress goes to stderr)
		done := make(chan bool, 2)

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Only print non-empty lines
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
			done <- true
		}()

		// Wait for both readers
		<-done
		<-done

		return cmd.Wait()
	}

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
	return AutoCommitVerbose(dir, false)
}

// AutoCommitVerbose stages all changes and creates a commit with optional output
func AutoCommitVerbose(dir string, verbose bool) error {
	if !HasChanges(dir) {
		return nil // Nothing to commit
	}

	if err := AddAll(dir); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	message := fmt.Sprintf("Deploy via cdp")
	return CommitVerbose(dir, message, verbose)
}

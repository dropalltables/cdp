package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/git"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:    "reset",
	Short:  "Reset project by deleting GitHub repo and Coolify project",
	Long:   "Deletes the GitHub repository and Coolify project associated with this project. Use with caution.",
	Hidden: true, // Debug command
	RunE:   runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	if err := checkLogin(); err != nil {
		return err
	}

	projectCfg, err := config.LoadProject()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}
	if projectCfg == nil {
		return fmt.Errorf("no cdp.json found")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show what will be deleted
	ui.Warning("This will DELETE the following resources:")
	ui.Spacer()
	if projectCfg.GitHubRepo != "" {
		ui.Dim(fmt.Sprintf("  Github repo: %s", projectCfg.GitHubRepo))
	}
	if projectCfg.ProjectUUID != "" {
		ui.Dim(fmt.Sprintf("  Coolify project UUID: %s", projectCfg.ProjectUUID))
	}
	if projectCfg.AppUUID != "" {
		ui.Dim(fmt.Sprintf("  Coolify app: %s", projectCfg.AppUUID))
	}
	ui.Spacer()

	confirm, err := ui.Confirm("Are you sure?")
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Double confirm
	confirm2, err := ui.Confirm("Really delete everything?")
	if err != nil {
		return err
	}
	if !confirm2 {
		return nil
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// Collect tasks for deletion
	tasks := []ui.Task{}

	// Delete Coolify app
	if projectCfg.AppUUID != "" {
		tasks = append(tasks, ui.Task{
			Name:         "delete-app",
			ActiveName:   "Deleting Coolify app...",
			CompleteName: "Deleted Coolify app",
			Action: func() error {
				return client.DeleteApplication(projectCfg.AppUUID)
			},
		})
	}

	// Delete Coolify project with retries
	if projectCfg.ProjectUUID != "" {
		projectUUID := projectCfg.ProjectUUID
		tasks = append(tasks, ui.Task{
			Name:         "delete-project",
			ActiveName:   "Deleting Coolify project...",
			CompleteName: "Deleted Coolify project",
			Action: func() error {
				// Wait for cleanup after app deletion
				time.Sleep(2 * time.Second)

				// Try up to 5 times with increasing delays
				var lastErr error
				for attempt := 1; attempt <= 5; attempt++ {
					err := client.DeleteProject(projectUUID)
					if err == nil {
						return nil
					}
					lastErr = err
					if attempt < 5 {
						time.Sleep(time.Duration(attempt*2) * time.Second)
					}
				}
				return lastErr
			},
		})
	}

	// Delete GitHub repo
	if projectCfg.GitHubRepo != "" && globalCfg.GitHubToken != "" {
		githubRepo := projectCfg.GitHubRepo
		githubToken := globalCfg.GitHubToken
		tasks = append(tasks, ui.Task{
			Name:         "delete-repo",
			ActiveName:   "Deleting GitHub repository...",
			CompleteName: "Deleted GitHub repository",
			Action: func() error {
				ghClient := git.NewGitHubClient(githubToken)
				user, err := ghClient.GetUser()
				if err != nil {
					return err
				}

				// Extract just the repo name
				repoName := githubRepo
				if strings.Contains(repoName, "/") {
					parts := strings.Split(repoName, "/")
					repoName = parts[len(parts)-1]
				}

				return ghClient.DeleteRepo(user.Login, repoName)
			},
		})
	}

	// Delete local files
	if _, err := os.Stat("cdp.json"); err == nil {
		tasks = append(tasks, ui.Task{
			Name:         "delete-config",
			ActiveName:   "Removing cdp.json...",
			CompleteName: "Removed cdp.json",
			Action: func() error {
				return config.DeleteProject()
			},
		})
	}

	if _, err := os.Stat("README.md"); err == nil {
		tasks = append(tasks, ui.Task{
			Name:         "delete-readme",
			ActiveName:   "Removing README.md...",
			CompleteName: "Removed README.md",
			Action: func() error {
				return os.Remove("README.md")
			},
		})
	}

	if _, err := os.Stat(".git"); err == nil {
		tasks = append(tasks, ui.Task{
			Name:         "delete-git",
			ActiveName:   "Removing .git directory...",
			CompleteName: "Removed .git directory",
			Action: func() error {
				return os.RemoveAll(".git")
			},
		})
	}

	// Run all deletion tasks
	if len(tasks) > 0 {
		if err := ui.RunTasks(tasks); err != nil {
			// Some tasks may fail (e.g., remote resources), but continue
			ui.Warning("Some resources could not be deleted, but local files have been cleaned")
		}
	}

	ui.Spacer()
	ui.Success("Reset complete.")

	return nil
}

package cmd

import (
	"fmt"
	"os"
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
	fmt.Println()
	ui.Warning("This will DELETE the following resources:")
	fmt.Println()
	if projectCfg.GitHubRepo != "" {
		fmt.Printf("  GitHub repo: %s\n", projectCfg.GitHubRepo)
	}
	if projectCfg.ProjectUUID != "" {
		fmt.Printf("  Coolify project UUID: %s\n", projectCfg.ProjectUUID)
	}
	if projectCfg.AppUUID != "" {
		fmt.Printf("  Coolify app: %s\n", projectCfg.AppUUID)
	}
	fmt.Println()

	confirm, err := ui.Confirm("Are you sure? This cannot be undone!")
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("cancelled")
	}

	// Double confirm
	confirm2, err := ui.Confirm("Really delete everything?")
	if err != nil {
		return err
	}
	if !confirm2 {
		return fmt.Errorf("cancelled")
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// Delete Coolify app
	if projectCfg.AppUUID != "" {
		ui.Info("Deleting Coolify app...")
		err := client.DeleteApplication(projectCfg.AppUUID)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to delete app: %v", err))
		} else {
			ui.Success("Deleted Coolify app")
		}
	}

	// Delete Coolify project with retries
	// (Coolify requires all resources to be deleted first)
	if projectCfg.ProjectUUID != "" {
		ui.Info("Waiting for Coolify cleanup...")
		time.Sleep(5 * time.Second)

		ui.Info("Deleting Coolify project...")

		// Try up to 5 times with increasing delays
		var lastErr error
		for attempt := 1; attempt <= 5; attempt++ {
			err := client.DeleteProject(projectCfg.ProjectUUID)
			if err == nil {
				ui.Success("Deleted Coolify project")
				break
			}

			lastErr = err
			if attempt < 5 {
				waitTime := time.Duration(attempt*2) * time.Second
				ui.Dim(fmt.Sprintf("Retry %d/5 in %ds...", attempt, int(waitTime.Seconds())))
				time.Sleep(waitTime)
			}
		}

		if lastErr != nil {
			ui.Warning(fmt.Sprintf("Failed to delete project: %v", lastErr))
			ui.Dim("You may need to manually delete the project in Coolify if it still has resources")
		}
	}

	// Delete GitHub repo
	if projectCfg.GitHubRepo != "" && globalCfg.GitHubToken != "" {
		ghClient := git.NewGitHubClient(globalCfg.GitHubToken)

		// Get current user to build full repo name
		user, err := ghClient.GetUser()
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to get GitHub user: %v", err))
		} else {
			ui.Info("Deleting GitHub repository...")
			err = ghClient.DeleteRepo(user.Login, projectCfg.GitHubRepo)
			if err != nil {
				ui.Warning(fmt.Sprintf("Failed to delete repo: %v", err))
			} else {
				ui.Success(fmt.Sprintf("Deleted GitHub repo: %s/%s", user.Login, projectCfg.GitHubRepo))
			}
		}
	}

	// Delete local cdp.json
	ui.Info("Removing cdp.json...")
	err = config.DeleteProject()
	if err != nil {
		ui.Warning(fmt.Sprintf("Failed to delete cdp.json: %v", err))
	} else {
		ui.Success("Removed cdp.json")
	}

	// Delete README.md if it exists
	if _, err := os.Stat("README.md"); err == nil {
		ui.Info("Removing README.md...")
		err = os.Remove("README.md")
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to delete README.md: %v", err))
		} else {
			ui.Success("Removed README.md")
		}
	}

	// Delete .git directory if it exists
	if _, err := os.Stat(".git"); err == nil {
		ui.Info("Removing .git directory...")
		err = os.RemoveAll(".git")
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to delete .git: %v", err))
		} else {
			ui.Success("Removed .git directory")
		}
	}

	fmt.Println()
	ui.Success("Reset complete. Run 'cdp' to set up again.")

	return nil
}

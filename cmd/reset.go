package cmd

import (
	"fmt"
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
	for env, uuid := range projectCfg.AppUUIDs {
		fmt.Printf("  Coolify app (%s): %s\n", env, uuid)
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

	// Delete Coolify apps first
	deletedApps := false
	for env, uuid := range projectCfg.AppUUIDs {
		if uuid == "" {
			continue
		}
		spinner := ui.NewSpinner(fmt.Sprintf("Deleting Coolify app (%s)...", env))
		spinner.Start()
		err := client.DeleteApplication(uuid)
		spinner.Stop()
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to delete app %s: %v", env, err))
		} else {
			ui.Success(fmt.Sprintf("Deleted Coolify app (%s)", env))
			deletedApps = true
		}
	}

	// Wait for Coolify to finish cleanup
	if deletedApps {
		spinner := ui.NewSpinner("Waiting for Coolify cleanup...")
		spinner.Start()
		time.Sleep(5 * time.Second)
		spinner.Stop()
	}

	// Delete Coolify project
	if projectCfg.ProjectUUID != "" {
		spinner := ui.NewSpinner("Deleting Coolify project...")
		spinner.Start()
		err := client.DeleteProject(projectCfg.ProjectUUID)
		spinner.Stop()
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to delete project: %v", err))
		} else {
			ui.Success("Deleted Coolify project")
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
			spinner := ui.NewSpinner("Deleting GitHub repository...")
			spinner.Start()
			err = ghClient.DeleteRepo(user.Login, projectCfg.GitHubRepo)
			spinner.Stop()
			if err != nil {
				ui.Warning(fmt.Sprintf("Failed to delete repo: %v", err))
			} else {
				ui.Success(fmt.Sprintf("Deleted GitHub repo: %s/%s", user.Login, projectCfg.GitHubRepo))
			}
		}
	}

	// Delete local cdp.json
	spinner := ui.NewSpinner("Removing cdp.json...")
	spinner.Start()
	err = config.DeleteProject()
	spinner.Stop()
	if err != nil {
		ui.Warning(fmt.Sprintf("Failed to delete cdp.json: %v", err))
	} else {
		ui.Success("Removed cdp.json")
	}

	fmt.Println()
	ui.Success("Reset complete. Run 'cdp' to set up again.")

	return nil
}

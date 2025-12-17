package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to a previous deployment",
	Long:  "List recent deployments and rollback to a previous version.",
	RunE:  runRollback,
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) error {
	if err := checkLogin(); err != nil {
		return err
	}

	projectCfg, err := config.LoadProject()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}
	if projectCfg == nil {
		return fmt.Errorf("not linked to a project. Run '%s' or '%s link' first", execName(), execName())
	}

	appUUID := projectCfg.AppUUIDs["preview"]
	envName := "preview"
	if prodFlag {
		appUUID = projectCfg.AppUUIDs["production"]
		envName = "production"
	}
	if appUUID == "" {
		return fmt.Errorf("no application found for %s environment. Deploy first with '%s'", envName, execName())
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// List recent deployments
	spinner := ui.NewSpinner("Fetching deployments...")
	spinner.Start()
	deployments, err := client.ListDeployments(appUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if len(deployments) < 2 {
		return fmt.Errorf("no previous deployments to rollback to")
	}

	// Show deployment options (skip the current one)
	fmt.Println()
	fmt.Printf("Recent deployments for %s:\n", envName)
	fmt.Println(strings.Repeat("-", 50))

	options := make(map[string]string)
	for i, d := range deployments {
		if i == 0 {
			continue // Skip current deployment
		}
		if i > 10 {
			break // Limit to last 10
		}

		commit := d.GitCommitSha
		if len(commit) > 7 {
			commit = commit[:7]
		}
		if commit == "" {
			commit = "unknown"
		}

		msg := d.CommitMessage
		if len(msg) > 40 {
			msg = msg[:40] + "..."
		}
		if msg == "" {
			msg = "(no message)"
		}

		displayName := fmt.Sprintf("%s - %s (%s)", commit, msg, d.Status)
		options[d.DeploymentUUID] = displayName
	}

	if len(options) == 0 {
		return fmt.Errorf("no previous deployments to rollback to")
	}

	selectedUUID, err := ui.SelectWithKeys("Select deployment to rollback to:", options)
	if err != nil {
		return err
	}

	// Find the selected deployment to get commit info
	var selectedDeployment *api.Deployment
	for _, d := range deployments {
		if d.DeploymentUUID == selectedUUID {
			selectedDeployment = &d
			break
		}
	}

	if selectedDeployment == nil {
		return fmt.Errorf("deployment not found")
	}

	// Confirm rollback
	commit := selectedDeployment.GitCommitSha
	if len(commit) > 7 {
		commit = commit[:7]
	}
	confirm, err := ui.Confirm(fmt.Sprintf("Rollback to commit %s?", commit))
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("cancelled")
	}

	// Trigger rollback by updating the app to use the old commit and redeploying
	spinner = ui.NewSpinner("Rolling back...")
	spinner.Start()

	// Update app to use the old commit
	if selectedDeployment.GitCommitSha != "" {
		err = client.UpdateApplication(appUUID, map[string]any{
			"git_commit_sha": selectedDeployment.GitCommitSha,
		})
		if err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to set rollback commit: %w", err)
		}
	}

	// Trigger deploy
	_, err = client.Deploy(appUUID, true) // force rebuild
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to trigger rollback deploy: %w", err)
	}

	ui.Success(fmt.Sprintf("Rollback to %s initiated", commit))
	fmt.Println()
	fmt.Println("Use 'cdp logs' to monitor the deployment.")

	return nil
}

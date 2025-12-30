package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/deploy"
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
	if err != nil || projectCfg == nil {
		ui.Error("No project configuration found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy", execName()),
		})
		return fmt.Errorf("not linked to a project")
	}

	if projectCfg.DeployMethod == config.DeployMethodDocker {
		ui.Error("Rollback is not supported for Docker-based deployments")
		ui.Dim("For Docker deployments, manually redeploy a previous image tag")
		return nil
	}

	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		ui.Error("No application found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy first", execName()),
		})
		return fmt.Errorf("no application found")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// Get deployment history from Coolify API
	var deployments []api.Deployment
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-history",
			ActiveName:   "Fetching deployment history...",
			CompleteName: "Fetched deployment history",
			Action: func() error {
				var err error
				deployments, err = client.ListDeploymentHistory(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch deployment history")
		return fmt.Errorf("failed to fetch deployment history: %w", err)
	}

	if len(deployments) < 2 {
		ui.Warning("No previous deployments available")
		ui.Dim("You need at least 2 deployments to rollback")
		return nil
	}

	// Show deployment options (skip the current one)
	ui.Dim("Select a deployment to rollback to:")

	var options []struct{ Key, Display string }
	for i, d := range deployments {
		if i == 0 {
			continue // Skip current deployment
		}
		if i > 10 {
			break // Limit to last 10
		}

		commit := d.GitCommitSha
		if commit == "" {
			commit = d.Commit
		}
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

		status := strings.ToLower(d.Status)
		statusDisplay := d.Status
		if status == "finished" {
			statusDisplay = ui.SuccessStyle.Render("✓")
		} else if status == "failed" {
			statusDisplay = ui.ErrorStyle.Render("✗")
		}

		displayName := fmt.Sprintf("%s  %s  %s", commit, msg, statusDisplay)
		options = append(options, struct{ Key, Display string }{Key: d.DeploymentUUID, Display: displayName})
	}

	if len(options) == 0 {
		ui.Warning("No previous deployments found")
		return nil
	}

	selectedUUID, err := ui.SelectWithKeysOrdered("", options)
	if err != nil {
		return err
	}

	// Find the selected deployment
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
	if commit == "" {
		commit = selectedDeployment.Commit
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}

	confirmed, err := ui.ConfirmAction("rollback to", commit)
	if err != nil {
		return err
	}
	if !confirmed {
		ui.Dim("Cancelled")
		return nil
	}

	// Trigger rollback by updating the git commit and deploying
	ui.Info("Initiating rollback...")
	fullCommit := selectedDeployment.GitCommitSha
	if fullCommit == "" {
		fullCommit = selectedDeployment.Commit
	}
	if fullCommit != "" {
		err = client.UpdateApplication(appUUID, map[string]any{
			"git_commit_sha": fullCommit,
		})
		if err != nil {
			ui.Error("Failed to update application")
			return fmt.Errorf("rollback failed: %w", err)
		}
	}

	// Deploy with force rebuild
	_, err = client.Deploy(appUUID, true, 0)
	if err != nil {
		ui.Error("Failed to trigger deployment")
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Watch deployment
	ui.Info("Watching deployment...")

	success := deploy.WatchDeployment(client, appUUID)

	if !success {
		ui.Error("Rollback failed")
		return fmt.Errorf("rollback failed")
	}

	ui.Success(fmt.Sprintf("Rolled back to %s", commit))

	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		fmt.Println(ui.DimStyle.Render("  URL: " + app.FQDN))
	}

	return nil
}

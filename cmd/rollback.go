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
	if err != nil || projectCfg == nil {
		ui.Error("No project configuration found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy", execName()),
		})
		return fmt.Errorf("not linked to a project")
	}

	if projectCfg.DeployMethod == config.DeployMethodDocker {
		ui.Error("Rollback is not supported for Docker-based deployments")
		ui.Spacer()
		ui.Dim("For Docker deployments, manually redeploy a previous image tag")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s --help' for usage", execName()),
		})
		return nil
	}

	envName := config.EnvPreview
	if prodFlag {
		envName = config.EnvProduction
	}

	appUUID := projectCfg.AppUUIDs[envName]
	if appUUID == "" {
		ui.Error(fmt.Sprintf("No deployment found for %s environment", envName))
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy first", execName()),
		})
		return fmt.Errorf("no deployment found")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	ui.Section(fmt.Sprintf("Rollback - %s", envName))

	// List recent deployments
	ui.Info("Fetching deployment history...")
	deployments, err := client.ListDeployments(appUUID)
	if err != nil {
		ui.Error("Failed to fetch deployments")
		return fmt.Errorf("failed to fetch deployments: %w", err)
	}
	ui.Success("Fetched deployment history")

	if len(deployments) < 2 {
		ui.Spacer()
		ui.Warning("No previous deployments available")
		ui.Dim("You need at least 2 deployments to rollback")
		return nil
	}

	// Show deployment options (skip the current one)
	ui.Spacer()
	ui.Dim("Select a deployment to rollback to:")
	ui.Spacer()

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

		status := d.Status
		if strings.ToLower(status) == "finished" {
			status = ui.SuccessStyle.Render(status)
		} else if strings.ToLower(status) == "failed" {
			status = ui.ErrorStyle.Render(status)
		}

		displayName := fmt.Sprintf("%s - %s [%s]", commit, msg, status)
		options[d.DeploymentUUID] = displayName
	}

	if len(options) == 0 {
		ui.Warning("No previous successful deployments found")
		return nil
	}

	selectedUUID, err := ui.SelectWithKeys("Choose deployment:", options)
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
	if len(commit) > 7 {
		commit = commit[:7]
	}

	ui.Spacer()
	confirmed, err := ui.ConfirmAction("rollback to", commit)
	if err != nil {
		return err
	}
	if !confirmed {
		ui.Dim("Cancelled")
		return nil
	}

	ui.Spacer()

	// Trigger rollback
	ui.Info("Initiating rollback...")
	if selectedDeployment.GitCommitSha != "" {
		err = client.UpdateApplication(appUUID, map[string]any{
			"git_commit_sha": selectedDeployment.GitCommitSha,
		})
		if err != nil {
			ui.Error("Failed to update application")
			return fmt.Errorf("rollback failed: %w", err)
		}
	}

	_, err = client.Deploy(appUUID, true)
	if err != nil {
		ui.Error("Failed to trigger deployment")
		return fmt.Errorf("rollback failed: %w", err)
	}

	ui.Spacer()
	ui.Success(fmt.Sprintf("Rollback to %s started", commit))

	ui.NextSteps([]string{
		fmt.Sprintf("Run '%s logs' to monitor deployment progress", execName()),
	})

	return nil
}

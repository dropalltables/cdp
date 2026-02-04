package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var cancelCmd = &cobra.Command{
	Use:   "cancel [deployment-uuid]",
	Short: "Cancel an in-progress deployment",
	Long:  "Cancel a deployment that is currently running. If no UUID is provided, cancels the latest deployment.",
	RunE:  runCancel,
}

func init() {
	rootCmd.AddCommand(cancelCmd)
}

func runCancel(cmd *cobra.Command, args []string) error {
	if err := checkLogin(); err != nil {
		return err
	}

	projectCfg, err := config.LoadProject()
	if err != nil || projectCfg == nil {
		ui.Error("No project configuration found")
		return fmt.Errorf("not linked to a project")
	}

	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		ui.Error("No application found")
		return fmt.Errorf("no application found")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	var deploymentUUID string
	if len(args) > 0 {
		deploymentUUID = args[0]
	} else {
		deployments, err := client.ListDeployments(appUUID)
		if err != nil {
			ui.Error("Failed to fetch deployments")
			return fmt.Errorf("failed to fetch deployments: %w", err)
		}

		for _, d := range deployments {
			if d.Status == "in_progress" || d.Status == "queued" || d.Status == "pending" {
				deploymentUUID = d.DeploymentUUID
				if deploymentUUID == "" {
					deploymentUUID = d.UUID
				}
				break
			}
		}

		if deploymentUUID == "" {
			ui.Info("No in-progress deployment found")
			return nil
		}
	}

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "cancel-deployment",
			ActiveName:   "Canceling deployment...",
			CompleteName: "Deployment canceled",
			Action: func() error {
				return client.CancelDeployment(deploymentUUID)
			},
		},
	})
	if err != nil {
		ui.Error("Failed to cancel deployment")
		return fmt.Errorf("failed to cancel deployment: %w", err)
	}

	ui.Success("Deployment canceled")
	return nil
}

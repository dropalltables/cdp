package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/deploy"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var (
	startForce bool
	startWatch bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the application",
	Long:  "Start the application, triggering a new deployment.",
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVarP(&startForce, "force", "f", false, "Force rebuild")
	startCmd.Flags().BoolVarP(&startWatch, "watch", "w", false, "Watch deployment progress")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
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

	var resp *api.LifecycleResponse
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "start-app",
			ActiveName:   "Starting application...",
			CompleteName: "Application started",
			Action: func() error {
				var err error
				resp, err = client.StartApplication(appUUID, startForce)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to start application")
		return fmt.Errorf("failed to start application: %w", err)
	}

	ui.Success(resp.Message)

	if startWatch && resp.DeploymentUUID != "" {
		ui.Info("Watching deployment...")
		result := deploy.WatchDeploymentWithCancel(client, appUUID)
		if result == deploy.WatchFailed {
			ui.Error("Deployment failed")
			return fmt.Errorf("deployment failed")
		}
		if result == deploy.WatchSuccess {
			ui.Success("Deployment complete")
		}
	}

	return nil
}

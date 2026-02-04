package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the application",
	Long:  "Stop the running application.",
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
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
			Name:         "stop-app",
			ActiveName:   "Stopping application...",
			CompleteName: "Application stopped",
			Action: func() error {
				var err error
				resp, err = client.StopApplication(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to stop application")
		return fmt.Errorf("failed to stop application: %w", err)
	}

	ui.Success(resp.Message)

	// Show confirmation
	ui.Dim("Run 'cdp start' to start the application again")

	return nil
}

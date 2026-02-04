package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the application",
	Long:  "Restart the running application.",
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
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
			Name:         "restart-app",
			ActiveName:   "Restarting application...",
			CompleteName: "Application restarted",
			Action: func() error {
				var err error
				resp, err = client.RestartApplication(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to restart application")
		return fmt.Errorf("failed to restart application: %w", err)
	}

	ui.Success(resp.Message)

	// Show current status
	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		ui.KeyValue("URL", app.FQDN)
	}

	return nil
}

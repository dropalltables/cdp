package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list", "status"},
	Short:   "List project deployments",
	Long:    "Display all environments and their deployment status for this project.",
	RunE:    runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	if err := checkLogin(); err != nil {
		return err
	}

	projectCfg, err := config.LoadProject()
	if err != nil || projectCfg == nil {
		ui.Error("No project configuration found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to set up a new project", execName()),
			fmt.Sprintf("Run '%s link' to link to an existing app", execName()),
		})
		return fmt.Errorf("not linked to a project")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		ui.Warning("No application found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy", execName()),
		})
		return nil
	}

	// Fetch application info
	var app *api.Application
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-app",
			ActiveName:   "Fetching application info...",
			CompleteName: "Fetched application info",
			Action: func() error {
				var err error
				app, err = client.GetApplication(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch application info")
		return fmt.Errorf("failed to fetch application: %w", err)
	}

	status := app.Status
	if status == "" {
		status = "unknown"
	}

	// Style status based on value
	var statusDisplay string
	statusLower := strings.ToLower(status)
	switch statusLower {
	case "running":
		statusDisplay = ui.SuccessStyle.Render(ui.IconSuccess + " " + status)
	case "stopped", "exited":
		statusDisplay = ui.DimStyle.Render(ui.IconDot + " " + status)
	case "starting", "restarting":
		statusDisplay = ui.InfoStyle.Render(ui.IconDot + " " + status)
	case "error", "failed":
		statusDisplay = ui.ErrorStyle.Render(ui.IconError + " " + status)
	default:
		statusDisplay = status
	}

	ui.KeyValue("Status", statusDisplay)

	url := app.FQDN
	if url != "" {
		ui.KeyValue("Production URL", ui.InfoStyle.Render(url))
	}

	if app.PreviewURLTemplate != "" {
		ui.KeyValue("Preview URL Template", ui.DimStyle.Render(app.PreviewURLTemplate))
	}

	if app.IsPreviewDeploymentsEnabled {
		ui.Spacer()
		ui.Success("Preview deployments enabled")
	}

	ui.Spacer()
	ui.KeyValue("Deploy method", projectCfg.DeployMethod)
	ui.KeyValue("Framework", projectCfg.Framework)

	return nil
}

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

	ui.Section(fmt.Sprintf("Project: %s", projectCfg.Name))

	if len(projectCfg.AppUUIDs) == 0 {
		ui.Warning("No deployments found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy", execName()),
		})
		return nil
	}

	// Fetch all apps
	type appInfo struct {
		env    string
		app    *api.Application
		err    error
	}

	apps := []appInfo{}
	for env, appUUID := range projectCfg.AppUUIDs {
		if appUUID == "" {
			continue
		}

		app, err := client.GetApplication(appUUID)
		apps = append(apps, appInfo{
			env: env,
			app: app,
			err: err,
		})
	}

	// Build table
	headers := []string{"Environment", "Status", "URL"}
	rows := [][]string{}

	for _, info := range apps {
		if info.err != nil {
			rows = append(rows, []string{
				info.env,
				ui.ErrorStyle.Render("error"),
				ui.DimStyle.Render("-"),
			})
			continue
		}

		status := info.app.Status
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

		url := info.app.FQDN
		if url == "" {
			url = ui.DimStyle.Render("-")
		} else {
			url = ui.InfoStyle.Render(url)
		}

		rows = append(rows, []string{
			info.env,
			statusDisplay,
			url,
		})
	}

	ui.Table(headers, rows)
	
	ui.Spacer()
	ui.KeyValue("Deploy method", projectCfg.DeployMethod)
	ui.KeyValue("Framework", projectCfg.Framework)

	return nil
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show application status",
	Long:  "Display detailed status information about the current application.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	var app *api.Application
	var deployments []api.Deployment
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-status",
			ActiveName:   "Fetching status...",
			CompleteName: "Fetched status",
			Action: func() error {
				var err error
				app, err = client.GetApplication(appUUID)
				if err != nil {
					return err
				}
				deployments, _ = client.ListDeploymentHistory(appUUID)
				return nil
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch status")
		return fmt.Errorf("failed to fetch status: %w", err)
	}

	ui.Spacer()
	ui.Bold(app.Name)
	ui.Spacer()

	status := app.Status
	if status == "" {
		status = "unknown"
	}

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

	if app.FQDN != "" {
		ui.KeyValue("URL", ui.InfoStyle.Render(app.FQDN))
	}

	if app.GitRepository != "" {
		ui.KeyValue("Repository", app.GitRepository)
		ui.KeyValue("Branch", app.GitBranch)
	}

	if app.DockerRegistryName != "" {
		tag := app.DockerRegistryTag
		if tag == "" {
			tag = "latest"
		}
		ui.KeyValue("Image", fmt.Sprintf("%s:%s", app.DockerRegistryName, tag))
	}

	ui.KeyValue("Port", app.PortsExposes)

	if app.BuildCommand != "" {
		ui.Spacer()
		ui.Dim("Build Configuration:")
		if app.InstallCommand != "" {
			ui.KeyValue("  Install", app.InstallCommand)
		}
		ui.KeyValue("  Build", app.BuildCommand)
		if app.StartCommand != "" {
			ui.KeyValue("  Start", app.StartCommand)
		}
	}

	if len(deployments) > 0 {
		ui.Spacer()
		ui.Dim("Recent Deployments:")
		count := len(deployments)
		if count > 5 {
			count = 5
		}
		for i := 0; i < count; i++ {
			d := deployments[i]
			statusIcon := ui.IconDot
			switch d.Status {
			case "finished":
				statusIcon = ui.IconSuccess
			case "failed", "error":
				statusIcon = ui.IconError
			case "in_progress", "queued":
				statusIcon = "◐"
			}
			commit := d.Commit
			if len(commit) > 7 {
				commit = commit[:7]
			}
			msg := d.CommitMessage
			if len(msg) > 40 {
				msg = msg[:40] + "..."
			}
			if commit != "" {
				ui.List([]string{fmt.Sprintf("%s %s %s - %s", statusIcon, d.Status, commit, msg)})
			} else {
				ui.List([]string{fmt.Sprintf("%s %s", statusIcon, d.Status)})
			}
		}
	}

	return nil
}

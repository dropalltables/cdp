package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var (
	logsRuntime bool
	logsLines   int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View application logs",
	Long:  "Display logs from the application. Use --runtime for runtime logs, otherwise shows deployment/build logs.",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsRuntime, "runtime", "r", false, "Show runtime logs instead of deployment logs")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of lines to show")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
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

	if logsRuntime {
		return showRuntimeLogs(client, appUUID)
	}
	return showDeploymentLogs(client, appUUID)
}

func showDeploymentLogs(client *api.Client, appUUID string) error {
	var logs string
	err := ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-logs",
			ActiveName:   "Fetching deployment logs...",
			CompleteName: "Fetched deployment logs",
			Action: func() error {
				var err error
				logs, err = client.GetDeploymentLogs(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch logs")
		return fmt.Errorf("failed to fetch logs: %w", err)
	}

	if logs == "" {
		ui.Dim("No deployment logs available yet")
		ui.Spacer()
		ui.NextSteps([]string{
			"Wait for deployment to start",
			fmt.Sprintf("Run '%s logs' again to check", execName()),
			fmt.Sprintf("Run '%s logs --runtime' for runtime logs", execName()),
		})
		return nil
	}

	ui.Spacer()
	logStream := ui.NewLogStream()
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line != "" {
			logStream.Write(line)
		}
	}

	return nil
}

func showRuntimeLogs(client *api.Client, appUUID string) error {
	var logs []api.ApplicationLog
	err := ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-logs",
			ActiveName:   "Fetching runtime logs...",
			CompleteName: "Fetched runtime logs",
			Action: func() error {
				var err error
				logs, err = client.GetApplicationLogs(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch runtime logs")
		return fmt.Errorf("failed to fetch logs: %w", err)
	}

	if len(logs) == 0 {
		ui.Dim("No runtime logs available")
		ui.Spacer()
		ui.NextSteps([]string{
			"Make sure the application is running",
			fmt.Sprintf("Run '%s status' to check application status", execName()),
		})
		return nil
	}

	ui.Spacer()
	logStream := ui.NewLogStream()
	start := 0
	if len(logs) > logsLines {
		start = len(logs) - logsLines
	}
	for i := start; i < len(logs); i++ {
		logStream.Write(logs[i].Output)
	}

	return nil
}

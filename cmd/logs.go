package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View deployment logs",
	Long:  "Display logs from the most recent deployment.",
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
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

	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		ui.Error("No application found")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s' to deploy", execName()),
		})
		return fmt.Errorf("no application found")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	ui.Section("Deployment Logs")

	ui.Info("Fetching logs...")
	logs, err := client.GetDeploymentLogs(appUUID)
	if err != nil {
		ui.Error("Failed to fetch logs")
		return fmt.Errorf("failed to fetch logs: %w", err)
	}

	if logs == "" {
		ui.Dim("No logs available yet")
		ui.Spacer()
		ui.NextSteps([]string{
			fmt.Sprintf("Wait for deployment to start"),
			fmt.Sprintf("Run '%s logs' again to check", execName()),
		})
		return nil
	}

	// Display logs
	ui.Spacer()
	logStream := ui.NewLogStream()
	
	// Process and display logs line by line
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if line != "" {
			logStream.WriteRaw(line + "\n")
		}
	}

	return nil
}

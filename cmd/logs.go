package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View deployment logs",
	Long:  "View logs for the deployed application.",
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
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}
	if projectCfg == nil {
		return fmt.Errorf("not linked to a project. Run '%s' or '%s link' first", execName(), execName())
	}

	appUUID := projectCfg.AppUUIDs[config.EnvPreview]
	if prodFlag {
		appUUID = projectCfg.AppUUIDs[config.EnvProduction]
	}
	if appUUID == "" {
		return fmt.Errorf("no application found for this environment. Deploy first with '%s'", execName())
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	spinner := ui.NewSpinner("Fetching logs...")
	spinner.Start()
	logs, err := client.GetDeploymentLogs(appUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if logs == "" {
		ui.Dim("No logs available")
		return nil
	}

	fmt.Println(logs)
	return nil
}

package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List deployments",
	Long:  "List all applications associated with this project.",
	RunE:  runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
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

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	fmt.Printf("Deployments for %s:\n\n", projectCfg.Name)
	fmt.Printf("%-12s %-10s %-40s\n", "ENV", "STATUS", "URL")
	fmt.Printf("%-12s %-10s %-40s\n", "---", "------", "---")

	for env, appUUID := range projectCfg.AppUUIDs {
		if appUUID == "" {
			continue
		}

		app, err := client.GetApplication(appUUID)
		if err != nil {
			ui.Dim(fmt.Sprintf("%-12s %-10s %-40s", env, "error", "-"))
			continue
		}

		status := app.Status
		if status == "" {
			status = "unknown"
		}

		url := app.FQDN
		if url == "" {
			url = "-"
		}

		fmt.Printf("%-12s %-10s %-40s\n", env, status, url)
	}

	return nil
}

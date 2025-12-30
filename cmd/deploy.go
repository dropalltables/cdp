package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/deploy"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the current directory to Coolify",
	Long: `Deploy the current project to Coolify.

Manual deploys always go to production.
Preview deployments are created automatically by Coolify from GitHub Pull Requests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploy()
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func runDeploy() error {
	if err := checkLogin(); err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	projectCfg, err := config.LoadProject()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load project configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	isFirstDeploy := false

	// First-time setup if no project config exists
	if projectCfg == nil {
		projectCfg, err = deploy.FirstTimeSetup(client, globalCfg)
		if err != nil {
			// Exit silently on interrupt
			if strings.Contains(err.Error(), "interrupted") {
				return nil
			}
			return err
		}
		isFirstDeploy = true
	}

	// All manual deploys go to production (PR 0)
	// Preview deployments are created automatically by Coolify from GitHub PRs
	prNumber := 0
	deploymentType := "production"

	// Confirm deployments (except first deploy)
	if !isFirstDeploy {
		confirmed, err := ui.Confirm("Deploy to production?")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	ui.Spacer()
	ui.KeyValue("Project", projectCfg.Name)
	ui.KeyValue("Type", deploymentType)
	ui.KeyValue("Method", projectCfg.DeployMethod)

	// Check verbose mode
	verbose := IsVerbose()

	// Deploy based on method
	if projectCfg.DeployMethod == config.DeployMethodDocker {
		return deploy.DeployDocker(client, globalCfg, projectCfg, prNumber, verbose)
	}
	return deploy.DeployGit(client, globalCfg, projectCfg, prNumber, verbose)
}

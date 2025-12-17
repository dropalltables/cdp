package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link this directory to an existing Coolify application",
	Long: `Link the current directory to an existing Coolify application.

This allows you to deploy to an app that was created in the Coolify dashboard.`,
	RunE: runLink,
}

func init() {
	rootCmd.AddCommand(linkCmd)
}

func runLink(cmd *cobra.Command, args []string) error {
	if err := checkLogin(); err != nil {
		return err
	}

	// Check if already linked
	if config.ProjectExists() {
		overwrite, err := ui.Confirm("Project is already linked. Overwrite?")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// List applications
	spinner := ui.NewSpinner("Loading applications...")
	spinner.Start()
	apps, err := client.ListApplications()
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	if len(apps) == 0 {
		return fmt.Errorf("no applications found. Create one in Coolify first or run '%s' to deploy", execName())
	}

	// Select application
	appOptions := make(map[string]string)
	appMap := make(map[string]api.Application)
	for _, app := range apps {
		displayName := app.Name
		if app.FQDN != "" {
			displayName = fmt.Sprintf("%s (%s)", app.Name, app.FQDN)
		}
		appOptions[app.UUID] = displayName
		appMap[app.UUID] = app
	}

	appUUID, err := ui.SelectWithKeys("Select application:", appOptions)
	if err != nil {
		return err
	}

	app := appMap[appUUID]

	// Determine deploy method based on app config
	deployMethod := "git"
	if app.DockerRegistryName != "" {
		deployMethod = "docker"
	}

	// Create project config
	projectCfg := &config.ProjectConfig{
		Name:           getWorkingDirName(),
		DeployMethod:   deployMethod,
		ServerUUID:     "", // We don't have this from the app listing
		AppUUIDs:       map[string]string{"preview": appUUID},
		Framework:      app.BuildPack,
		InstallCommand: app.InstallCommand,
		BuildCommand:   app.BuildCommand,
		StartCommand:   app.StartCommand,
	}

	if app.DockerRegistryName != "" {
		projectCfg.DockerImage = app.DockerRegistryName
	}
	if app.GitRepository != "" {
		projectCfg.GitHubRepo = app.GitRepository
	}

	if err := config.SaveProject(projectCfg); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	ui.Success(fmt.Sprintf("Linked to %s", app.Name))
	ui.Dim(fmt.Sprintf("Run '%s' to deploy", execName()))

	return nil
}

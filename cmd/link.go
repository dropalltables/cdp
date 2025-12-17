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

	ui.Section("Link Project")

	// Check if already linked
	if config.ProjectExists() {
		ui.Warning("This directory is already linked to a project")
		ui.Spacer()
		overwrite, err := ui.Confirm("Overwrite existing configuration?")
		if err != nil {
			return err
		}
		if !overwrite {
			ui.Dim("Cancelled")
			return nil
		}
		ui.Spacer()
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// List applications
	var apps []api.Application
	err = ui.WithSpinner("Loading applications", func() error {
		var err error
		apps, err = client.ListApplications()
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	if len(apps) == 0 {
		ui.Spacer()
		ui.Warning("No applications found in Coolify")
		ui.NextSteps([]string{
			"Create an application in Coolify first, or",
			fmt.Sprintf("Run '%s' to create and deploy a new app", execName()),
		})
		return fmt.Errorf("no applications found")
	}

	// Select application
	ui.Spacer()
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

	appUUID, err := ui.SelectWithKeys("Select application to link:", appOptions)
	if err != nil {
		return err
	}

	app := appMap[appUUID]

	// Determine deploy method based on app config
	deployMethod := config.DeployMethodGit
	if app.DockerRegistryName != "" {
		deployMethod = config.DeployMethodDocker
	}

	// Determine which environment to link to
	env := config.EnvPreview
	if prodFlag {
		env = config.EnvProduction
	}

	// Create project config
	projectCfg := &config.ProjectConfig{
		Name:           getWorkingDirName(),
		DeployMethod:   deployMethod,
		ServerUUID:     "",
		AppUUIDs:       map[string]string{env: appUUID},
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
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	ui.Spacer()
	ui.Success("Project linked successfully")
	ui.Spacer()
	ui.KeyValue("Application", app.Name)
	ui.KeyValue("Environment", env)
	ui.KeyValue("Deploy method", deployMethod)

	ui.NextSteps([]string{
		fmt.Sprintf("Run '%s' to deploy to this application", execName()),
		fmt.Sprintf("Run '%s ls' to view deployment status", execName()),
	})

	return nil
}

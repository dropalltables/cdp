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
		ui.Warning("This directory is already linked to a project")
		ui.Spacer()
		overwrite, err := ui.Confirm("Overwrite existing configuration?")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// List applications
	var apps []api.Application
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "list-apps",
			ActiveName:   "Loading applications...",
			CompleteName: "Loaded applications",
			Action: func() error {
				var err error
				apps, err = client.ListApplications()
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to load applications")
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
	deployMethod := config.DeployMethodGit
	if app.DockerRegistryName != "" {
		deployMethod = config.DeployMethodDocker
	}

	// Find the project UUID for this app by checking all projects
	var projectUUID string
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "lookup-project",
			ActiveName:   "Looking up project information...",
			CompleteName: "Found project information",
			Action: func() error {
				projects, err := client.ListProjects()
				if err != nil {
					return nil // Non-fatal - project UUID is optional
				}
				for _, proj := range projects {
					// Check if this project has an environment that matches our app's environment
					projDetail, err := client.GetProject(proj.UUID)
					if err == nil && projDetail != nil {
						for _, env := range projDetail.Environments {
							if env.ID == app.EnvironmentID {
								projectUUID = proj.UUID
								return nil
							}
						}
					}
				}
				return nil
			},
		},
	})
	if err != nil {
		return err
	}

	// Create project config
	projectCfg := &config.ProjectConfig{
		Name:            getWorkingDirName(),
		DeployMethod:    deployMethod,
		ServerUUID:      "",
		ProjectUUID:     projectUUID, // Set if we found it
		AppUUID:         appUUID,
		EnvironmentUUID: "", // Will be fetched from app if needed
		Framework:       app.BuildPack,
		InstallCommand:  app.InstallCommand,
		BuildCommand:    app.BuildCommand,
		StartCommand:    app.StartCommand,
	}

	if app.DockerRegistryName != "" {
		projectCfg.DockerImage = app.DockerRegistryName
	}
	if app.GitRepository != "" {
		projectCfg.GitHubRepo = app.GitRepository
	}

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "save-config",
			ActiveName:   "Saving configuration...",
			CompleteName: "Project linked successfully",
			Action: func() error {
				return config.SaveProject(projectCfg)
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	ui.Spacer()
	ui.KeyValue("Application", app.Name)
	ui.KeyValue("Deploy method", deployMethod)

	return nil
}

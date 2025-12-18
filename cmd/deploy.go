package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/detect"
	"github.com/dropalltables/cdp/internal/docker"
	"github.com/dropalltables/cdp/internal/git"
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
		ui.Section("New Project Setup")
		ui.Dim("Let's configure your project for deployment")
		ui.Spacer()

		projectCfg, err = firstTimeSetup(client, globalCfg)
		if err != nil {
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
		ui.Spacer()
		confirmed, err := ui.Confirm("Deploy to production?")
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Dim("Deployment cancelled")
			return nil
		}
	}

	// Show deployment summary
	ui.Section("Deploy")
	ui.KeyValue("Project", projectCfg.Name)
	ui.KeyValue("Type", deploymentType)
	ui.KeyValue("Method", projectCfg.DeployMethod)
	ui.Spacer()

	// Deploy based on method
	if projectCfg.DeployMethod == config.DeployMethodDocker {
		return deployDocker(client, globalCfg, projectCfg, prNumber)
	}
	return deployGit(client, globalCfg, projectCfg, prNumber)
}

func firstTimeSetup(client *api.Client, globalCfg *config.GlobalConfig) (*config.ProjectConfig, error) {
	ui.StepProgress(1, 5, "Framework Detection")
	ui.Spacer()

	// Detect framework
	ui.Info("Analyzing project...")
	framework, err := detect.Detect(".")
	if err != nil {
		ui.Error("Failed to analyze project")
		return nil, fmt.Errorf("failed to detect framework: %w", err)
	}

	ui.Spacer()
	ui.Success(fmt.Sprintf("Detected: %s", framework.Name))
	ui.Spacer()

	// Display build settings
	ui.Dim("Build Configuration:")
	if framework.InstallCommand != "" {
		ui.KeyValue("  Install", ui.CodeStyle.Render(framework.InstallCommand))
	}
	if framework.BuildCommand != "" {
		ui.KeyValue("  Build", ui.CodeStyle.Render(framework.BuildCommand))
	}
	if framework.StartCommand != "" {
		ui.KeyValue("  Start", ui.CodeStyle.Render(framework.StartCommand))
	}
	if framework.PublishDirectory != "" {
		ui.KeyValue("  Publish dir", framework.PublishDirectory)
	}
	ui.Spacer()

	editSettings, err := ui.Confirm("Customize build settings?")
	if err != nil {
		return nil, err
	}
	if editSettings {
		ui.Spacer()
		framework, err = editBuildSettings(framework)
		if err != nil {
			return nil, err
		}
		ui.Spacer()
	}

	// Choose deployment method
	ui.Divider()
	ui.StepProgress(2, 5, "Deployment Method")
	ui.Spacer()

	deployMethod, err := chooseDeployMethod(globalCfg)
	if err != nil {
		return nil, err
	}
	ui.Spacer()
	// Display friendly name
	deployMethodDisplay := "Git"
	if deployMethod == config.DeployMethodDocker {
		deployMethodDisplay = "Docker"
	}
	ui.Dim(fmt.Sprintf("→ %s", deployMethodDisplay))

	// Select server
	ui.Divider()
	ui.StepProgress(3, 5, "Server Selection")
	ui.Spacer()

	var servers []api.Server
	var projects []api.Project

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "load-servers",
			ActiveName:   "Loading servers...",
			CompleteName: "✓ Loaded servers",
			Action: func() error {
				var err error
				servers, err = client.ListServers()
				return err
			},
		},
	})
	if err != nil {
		ui.Spacer()
		ui.Error("Failed to load servers")
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	if len(servers) == 0 {
		ui.Error("No servers found in Coolify")
		ui.Spacer()
		ui.Dim("Add a server in your Coolify dashboard first")
		return nil, fmt.Errorf("no servers available")
	}

	ui.Spacer()
	serverOptions := make(map[string]string)
	var selectedServerName string
	for _, s := range servers {
		displayName := s.Name
		if s.IP != "" {
			displayName = fmt.Sprintf("%s (%s)", s.Name, s.IP)
		}
		serverOptions[s.UUID] = displayName
	}
	serverUUID, err := ui.SelectWithKeys("Select server:", serverOptions)
	if err != nil {
		return nil, err
	}
	selectedServerName = serverOptions[serverUUID]
	ui.Spacer()
	ui.Dim(fmt.Sprintf("→ %s", selectedServerName))

	// Select or create project (questions only)
	ui.Divider()
	ui.StepProgress(4, 5, "Project Configuration")
	ui.Spacer()

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "load-projects",
			ActiveName:   "Loading projects...",
			CompleteName: "✓ Loaded projects",
			Action: func() error {
				var err error
				projects, err = client.ListProjects()
				return err
			},
		},
	})
	if err != nil {
		ui.Spacer()
		ui.Error("Failed to load projects")
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	ui.Spacer()
	projectOptions := make([]string, 0, len(projects)+1)
	projectOptions = append(projectOptions, "+ Create new project")
	projectMap := make(map[string]api.Project)
	for _, p := range projects {
		projectOptions = append(projectOptions, p.Name)
		projectMap[p.Name] = p
	}

	selectedProject, err := ui.Select("Select or create project:", projectOptions)
	if err != nil {
		return nil, err
	}

	var projectUUID string
	var environmentUUID string
	var newProjectName string
	var displayProjectName string
	needsProjectCreation := selectedProject == "+ Create new project"

	if needsProjectCreation {
		// Ask for project name
		ui.Spacer()
		projectName := getWorkingDirName()
		newProjectName, err = ui.InputWithDefault("Project name:", projectName)
		if err != nil {
			return nil, err
		}
		displayProjectName = newProjectName
	} else {
		// Use existing project
		project := projectMap[selectedProject]
		projectUUID = project.UUID
		displayProjectName = selectedProject

		// Will check/create environment at the end with other API calls
	}
	ui.Spacer()
	ui.Dim(fmt.Sprintf("→ %s", displayProjectName))

	// Advanced options
	ui.Divider()
	ui.StepProgress(5, 5, "Advanced Configuration")
	ui.Spacer()

	configureAdvanced, err := ui.Confirm("Configure advanced options?")
	if err != nil {
		return nil, err
	}

	port := framework.Port
	if port == "" {
		port = config.DefaultPort
	}
	platform := config.DefaultPlatform
	branch := config.DefaultBranch
	domain := ""

	if configureAdvanced {
		ui.Spacer()
		ui.Dim("Leave blank to use defaults")
		ui.Spacer()

		// Port
		port, err = ui.InputWithDefault("Application port:", port)
		if err != nil {
			return nil, err
		}
		ui.Spacer()
		ui.Dim(fmt.Sprintf("→ Port: %s", port))

		// Platform (for Docker builds)
		if deployMethod == config.DeployMethodDocker {
			ui.Spacer()
			platformOptions := []string{"linux/amd64 (Intel/AMD)", "linux/arm64 (ARM)"}
			platformChoice, err := ui.Select("Target platform:", platformOptions)
			if err != nil {
				return nil, err
			}
			if strings.Contains(platformChoice, "arm64") {
				platform = "linux/arm64"
			}
			ui.Spacer()
			ui.Dim(fmt.Sprintf("→ %s", platformChoice))
		}

		// Branch (for Git deploys)
		if deployMethod == config.DeployMethodGit {
			ui.Spacer()
			branch, err = ui.InputWithDefault("Git branch:", branch)
			if err != nil {
				return nil, err
			}
			ui.Spacer()
			ui.Dim(fmt.Sprintf("→ Branch: %s", branch))
		}

		// Domain
		ui.Spacer()
		useDomain, err := ui.Confirm("Configure custom domain?")
		if err != nil {
			return nil, err
		}
		if useDomain {
			ui.Spacer()
			domain, err = ui.Input("Domain:", "app.example.com")
			if err != nil {
				return nil, err
			}
			ui.Spacer()
			ui.Dim(fmt.Sprintf("→ %s", domain))
		}
	}

	// Create project config
	projectCfg := &config.ProjectConfig{
		Name:            getWorkingDirName(),
		DeployMethod:    deployMethod,
		ProjectUUID:     projectUUID,
		ServerUUID:      serverUUID,
		EnvironmentUUID: environmentUUID,
		AppUUID:         "", // Will be created on first deployment
		Framework:       framework.Name,
		BuildPack:       framework.BuildPack,
		InstallCommand:  framework.InstallCommand,
		BuildCommand:    framework.BuildCommand,
		StartCommand:    framework.StartCommand,
		PublishDir:      framework.PublishDirectory,
		Port:            port,
		Platform:        platform,
		Branch:          branch,
		Domain:          domain,
	}
	
	// Store project creation info if needed (will be created during deployment)
	if needsProjectCreation {
		projectCfg.Name = newProjectName
		projectCfg.ProjectUUID = "" // Signal that it needs creation
	}

	// Set up based on deploy method
	if deployMethod == config.DeployMethodDocker {
		if globalCfg.DockerRegistry != nil {
			projectCfg.DockerImage = docker.GetImageFullName(
				globalCfg.DockerRegistry.URL,
				globalCfg.DockerRegistry.Username,
				projectCfg.Name,
			)
		}
	} else {
		projectCfg.GitHubRepo = git.GenerateRepoName(projectCfg.Name)
	}

	// Save project config
	ui.Spacer()
	ui.Info("Saving configuration...")
	err = config.SaveProject(projectCfg)
	if err != nil {
		ui.Error("Failed to save configuration")
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}
	ui.Success("Saved configuration")

	ui.Spacer()
	ui.Divider()
	ui.Success("Project configured successfully")
	ui.Spacer()
	ui.KeyValue("Name", projectCfg.Name)
	ui.KeyValue("Framework", projectCfg.Framework)
	ui.KeyValue("Deploy method", projectCfg.DeployMethod)
	ui.Spacer()

	return projectCfg, nil
}

func editBuildSettings(f *detect.FrameworkInfo) (*detect.FrameworkInfo, error) {
	installCmd, err := ui.InputWithDefault("Install command:", f.InstallCommand)
	if err != nil {
		return nil, err
	}
	f.InstallCommand = installCmd

	buildCmd, err := ui.InputWithDefault("Build command:", f.BuildCommand)
	if err != nil {
		return nil, err
	}
	f.BuildCommand = buildCmd

	startCmd, err := ui.InputWithDefault("Start command:", f.StartCommand)
	if err != nil {
		return nil, err
	}
	f.StartCommand = startCmd

	return f, nil
}

func chooseDeployMethod(globalCfg *config.GlobalConfig) (string, error) {
	options := []string{}
	optionMap := map[string]string{}

	// Check what's available
	hasDocker := docker.IsDockerAvailable() && globalCfg.DockerRegistry != nil
	hasGitHub := globalCfg.GitHubToken != ""

	if hasGitHub {
		options = append(options, "Git (recommended)")
		optionMap["Git (recommended)"] = config.DeployMethodGit
	}
	if hasDocker {
		options = append(options, "Docker (build locally)")
		optionMap["Docker (build locally)"] = config.DeployMethodDocker
	}

	if len(options) == 0 {
		ui.Error("No deployment methods available")
		ui.Spacer()
		ui.Dim("Configure at least one deployment method:")
		ui.List([]string{
			"GitHub token (for git-based deployments)",
			"Docker registry (for container deployments)",
		})
		ui.Spacer()
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s login' to configure authentication", execName()),
		})
		return "", fmt.Errorf("no deployment method configured")
	}

	if len(options) == 1 {
		// Auto-select if only one option available
		return optionMap[options[0]], nil
	}

	// Show options
	selected, err := ui.Select("Choose deployment method:", options)
	if err != nil {
		return "", err
	}
	return optionMap[selected], nil
}

func deployDocker(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, prNumber int) error {
	// Generate tag based on PR number (0 = production, >0 = preview)
	deployType := "production"
	if prNumber > 0 {
		deployType = fmt.Sprintf("pr-%d", prNumber)
	}
	tag := docker.GenerateTag(deployType)
	
	needsProjectCreation := projectCfg.ProjectUUID == ""

	ui.Divider()
	ui.Bold("Docker Build")
	ui.Spacer()
	ui.KeyValue("Image", projectCfg.DockerImage)
	ui.KeyValue("Tag", tag)
	ui.KeyValue("Platform", projectCfg.Platform)
	ui.Spacer()

	// Build image
	framework := &detect.FrameworkInfo{
		Name:             projectCfg.Framework,
		InstallCommand:   projectCfg.InstallCommand,
		BuildCommand:     projectCfg.BuildCommand,
		StartCommand:     projectCfg.StartCommand,
		PublishDirectory: projectCfg.PublishDir,
	}

	// Use spinner for build unless verbose mode is enabled
	verbose := IsVerbose()
	var err error
	if !verbose {
		buildTask := ui.Task{
			Name:         "build-image",
			ActiveName:   "Building Docker image...",
			CompleteName: "✓ Image built successfully",
			Action: func() error {
				return docker.Build(&docker.BuildOptions{
					Dir:       ".",
					ImageName: projectCfg.DockerImage,
					Tag:       tag,
					Framework: framework,
					Platform:  projectCfg.Platform,
					Verbose:   false,
				})
			},
		}
		err = ui.RunTasks([]ui.Task{buildTask})
	} else {
		// In verbose mode, show build output directly
		ui.Info("Building Docker image...")
		ui.Spacer()
		err = docker.Build(&docker.BuildOptions{
			Dir:       ".",
			ImageName: projectCfg.DockerImage,
			Tag:       tag,
			Framework: framework,
			Platform:  projectCfg.Platform,
			Verbose:   true,
		})
		ui.Spacer()
		if err == nil {
			ui.Success("Image built successfully")
		}
	}

	if err != nil {
		ui.Error("Build failed")
		return fmt.Errorf("build failed: %w", err)
	}

	// ===== API Operations with BubbleTea task runner =====

	ui.Spacer()
	ui.Divider()
	ui.Spacer()

	// Build task list dynamically based on what needs to be created
	tasks := []ui.Task{}

	// Create project and environment if needed
	if needsProjectCreation {
		tasks = append(tasks, ui.Task{
			Name:         "create-project",
			ActiveName:   "Creating Coolify project...",
			CompleteName: "✓ Created Coolify project",
			Action: func() error {
				newProject, err := client.CreateProject(projectCfg.Name, "Created by CDP")
				if err != nil {
					return fmt.Errorf("failed to create Coolify project %q: %w", projectCfg.Name, err)
				}
				projectCfg.ProjectUUID = newProject.UUID
				return nil
			},
		})

		tasks = append(tasks, ui.Task{
			Name:         "setup-env",
			ActiveName:   "Setting up environment...",
			CompleteName: "✓ Set up environment",
			Action: func() error {
				// Fetch project to check for auto-created environments
				project, err := client.GetProject(projectCfg.ProjectUUID)
				if err == nil {
					for _, env := range project.Environments {
						if strings.ToLower(env.Name) == "production" {
							projectCfg.EnvironmentUUID = env.UUID
							break
						}
					}
				}

				// Create production environment if missing
				if projectCfg.EnvironmentUUID == "" {
					prodEnv, err := client.CreateEnvironment(projectCfg.ProjectUUID, "production")
					if err != nil {
						return fmt.Errorf("failed to create production environment: %w", err)
					}
					projectCfg.EnvironmentUUID = prodEnv.UUID
				}

				return config.SaveProject(projectCfg)
			},
		})
	} else {
		// Check if environment exists for existing project
		tasks = append(tasks, ui.Task{
			Name:         "check-env",
			ActiveName:   "Checking environment...",
			CompleteName: "✓ Environment ready",
			Action: func() error {
				if projectCfg.EnvironmentUUID == "" {
					project, err := client.GetProject(projectCfg.ProjectUUID)
					if err == nil {
						for _, env := range project.Environments {
							if strings.ToLower(env.Name) == "production" {
								projectCfg.EnvironmentUUID = env.UUID
								break
							}
						}
					}

					// Create if still missing
					if projectCfg.EnvironmentUUID == "" {
						prodEnv, err := client.CreateEnvironment(projectCfg.ProjectUUID, "production")
						if err != nil && !api.IsConflict(err) {
							return err
						}
						if prodEnv != nil {
							projectCfg.EnvironmentUUID = prodEnv.UUID
						}
					}

					return config.SaveProject(projectCfg)
				}
				return nil
			},
		})
	}

	// Add push image task
	tasks = append(tasks, ui.Task{
		Name:         "push-image",
		ActiveName:   "Pushing image to registry...",
		CompleteName: "✓ Pushed image to registry",
		Action: func() error {
			err := docker.Push(&docker.PushOptions{
				ImageName: projectCfg.DockerImage,
				Tag:       tag,
				Registry:  globalCfg.DockerRegistry.URL,
				Username:  globalCfg.DockerRegistry.Username,
				Password:  globalCfg.DockerRegistry.Password,
				Verbose:   verbose,
			})
			if err != nil {
				return fmt.Errorf("failed to push image %s:%s to registry: %w", projectCfg.DockerImage, tag, err)
			}
			return nil
		},
	})

	// Create app if needed
	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		tasks = append(tasks, ui.Task{
			Name:         "create-app",
			ActiveName:   "Creating Coolify application...",
			CompleteName: "✓ Created Coolify application",
			Action: func() error {
				port := projectCfg.Port
				if port == "" {
					port = config.DefaultPort
				}

				resp, err := client.CreateDockerImageApp(&api.CreateDockerImageAppRequest{
					ProjectUUID:             projectCfg.ProjectUUID,
					ServerUUID:              projectCfg.ServerUUID,
					EnvironmentUUID:         projectCfg.EnvironmentUUID,
					Name:                    projectCfg.Name,
					DockerRegistryImageName: projectCfg.DockerImage,
					DockerRegistryImageTag:  tag,
					PortsExposes:            port,
					InstantDeploy:           false,
				})
				if err != nil {
					return fmt.Errorf("failed to create Coolify application %q: %w", projectCfg.Name, err)
				}
				appUUID = resp.UUID
				projectCfg.AppUUID = appUUID

				return config.SaveProject(projectCfg)
			},
		})
	}

	// Trigger deployment
	tasks = append(tasks, ui.Task{
		Name:         "trigger-deploy",
		ActiveName:   "Triggering deployment...",
		CompleteName: "✓ Triggered deployment",
		Action: func() error {
			if err := client.UpdateApplication(appUUID, map[string]interface{}{
				"docker_registry_image_tag": tag,
			}); err != nil {
				return fmt.Errorf("failed to update application image tag: %w", err)
			}

			_, err := client.Deploy(appUUID, false, prNumber)
			if err != nil {
				return fmt.Errorf("failed to trigger deployment: %w", err)
			}
			return nil
		},
	})

	// Run all tasks
	if err := ui.RunTasksVerbose(tasks, verbose); err != nil {
		ui.Spacer()
		ui.Error("Deployment setup failed")
		return err
	}

	// ===== Watch deployment (keep logs as-is) =====
	
	ui.Spacer()
	ui.Info("Watching deployment...")

	success := watchDeployment(client, appUUID)

	ui.Spacer()

	if !success {
		ui.Error("Deployment failed")
		ui.Spacer()
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s logs' to view deployment logs", execName()),
			"Check the Coolify dashboard for more details",
		})
		return fmt.Errorf("deployment failed")
	}

	// Get app info for URL
	ui.Success("Deployment complete")
	ui.Spacer()

	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		ui.KeyValue("URL", ui.InfoStyle.Render(app.FQDN))
	}

	ui.Spacer()
	return nil
}

func deployGit(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, prNumber int) error {
	ghClient := git.NewGitHubClient(globalCfg.GitHubToken)

	ui.Divider()
	ui.Bold("Git Deployment")
	ui.Spacer()

	// Check verbose mode
	verbose := IsVerbose()

	// ===== PHASE 1: Questions =====

	// Create project/environment if needed (spinner at end)
	needsProjectCreation := projectCfg.ProjectUUID == ""
	
	// Get GitHub username
	var user *git.User
	err := ui.RunTasksVerbose([]ui.Task{
		{
			Name:         "github-check",
			ActiveName:   "Checking GitHub connection...",
			CompleteName: "✓ Connected to GitHub",
			Action: func() error {
				var err error
				user, err = ghClient.GetUser()
				return err
			},
		},
	}, verbose)
	if err != nil {
		ui.Spacer()
		ui.Error("Failed to connect to GitHub")
		return fmt.Errorf("failed to connect to GitHub: %w", err)
	}

	repoName := projectCfg.GitHubRepo
	fullRepoName := fmt.Sprintf("%s/%s", user.Login, repoName)

	// Check if repo needs to be created and ask questions
	needsRepoCreation := !ghClient.RepoExists(user.Login, repoName)
	var isPrivate bool
	
	if needsRepoCreation {
		ui.Spacer()
		ui.Bold("GitHub Repository Setup")
		ui.Spacer()

		// Ask for repo name
		repoName, err = ui.InputWithDefault("Repository name:", repoName)
		if err != nil {
			return err
		}
		projectCfg.GitHubRepo = repoName
		fullRepoName = fmt.Sprintf("%s/%s", user.Login, repoName)
		ui.Spacer()
		ui.Dim(fmt.Sprintf("→ %s", fullRepoName))

		// Ask for visibility
		ui.Spacer()
		visibilityOptions := []string{"Private", "Public"}
		visibility, err := ui.Select("Repository visibility:", visibilityOptions)
		if err != nil {
			return err
		}
		isPrivate = visibility == "Private"
		projectCfg.GitHubPrivate = isPrivate
		ui.Spacer()
		ui.Dim(fmt.Sprintf("→ %s", visibility))
	}

	// Get GitHub App for Coolify (question if multiple)
	ui.Spacer()
	var githubApps []api.GitHubApp
	err = ui.RunTasksVerbose([]ui.Task{
		{
			Name:         "load-apps",
			ActiveName:   "Loading GitHub Apps...",
			CompleteName: "✓ Loaded GitHub Apps",
			Action: func() error {
				var err error
				githubApps, err = client.ListGitHubApps()
				return err
			},
		},
	}, verbose)
	if err != nil {
		ui.Spacer()
		ui.Error("Failed to load GitHub Apps")
		ui.Spacer()
		ui.Dim("Configure a GitHub App in Coolify: Sources → GitHub App")
		return fmt.Errorf("failed to list GitHub Apps: %w", err)
	}
	
	if len(githubApps) == 0 {
		ui.Error("No GitHub Apps configured in Coolify")
		ui.Spacer()
		ui.Dim("Add a GitHub App in Coolify: Sources → GitHub App")
		return fmt.Errorf("no GitHub Apps configured")
	}

	// Select GitHub App (prompt if multiple)
	var githubAppUUID string
	var selectedAppName string
	if len(githubApps) == 1 {
		githubAppUUID = githubApps[0].UUID
		selectedAppName = githubApps[0].Name
	} else {
		ui.Spacer()
		appOptions := make(map[string]string)
		for _, app := range githubApps {
			displayName := app.Name
			if app.Organization != "" {
				displayName = fmt.Sprintf("%s (%s)", app.Name, app.Organization)
			}
			appOptions[app.UUID] = displayName
		}
		githubAppUUID, err = ui.SelectWithKeys("Select GitHub App:", appOptions)
		if err != nil {
			return err
		}
		selectedAppName = appOptions[githubAppUUID]
	}
	ui.Spacer()
	ui.Dim(fmt.Sprintf("→ %s", selectedAppName))

	// ===== PHASE 2: API Operations with BubbleTea task runner =====

	ui.Spacer()
	ui.Divider()
	ui.Spacer()

	// Build task list dynamically
	tasks := []ui.Task{}

	// Create project and environment if needed
	if needsProjectCreation {
		tasks = append(tasks, ui.Task{
			Name:         "create-project",
			ActiveName:   "Creating Coolify project...",
			CompleteName: "✓ Created Coolify project",
			Action: func() error {
				newProject, err := client.CreateProject(projectCfg.Name, "Created by CDP")
				if err != nil {
					return fmt.Errorf("failed to create Coolify project %q: %w", projectCfg.Name, err)
				}
				projectCfg.ProjectUUID = newProject.UUID
				return nil
			},
		})

		tasks = append(tasks, ui.Task{
			Name:         "setup-env",
			ActiveName:   "Setting up environment...",
			CompleteName: "✓ Set up environment",
			Action: func() error {
				// Fetch project to check for auto-created environments
				project, err := client.GetProject(projectCfg.ProjectUUID)
				if err == nil {
					for _, env := range project.Environments {
						if strings.ToLower(env.Name) == "production" {
							projectCfg.EnvironmentUUID = env.UUID
							break
						}
					}
				}

				// Create production environment if missing
				if projectCfg.EnvironmentUUID == "" {
					prodEnv, err := client.CreateEnvironment(projectCfg.ProjectUUID, "production")
					if err != nil {
						return fmt.Errorf("failed to create production environment: %w", err)
					}
					projectCfg.EnvironmentUUID = prodEnv.UUID
				}

				return config.SaveProject(projectCfg)
			},
		})
	} else {
		// Check if environment exists for existing project
		tasks = append(tasks, ui.Task{
			Name:         "check-env",
			ActiveName:   "Checking environment...",
			CompleteName: "✓ Environment ready",
			Action: func() error {
				if projectCfg.EnvironmentUUID == "" {
					project, err := client.GetProject(projectCfg.ProjectUUID)
					if err == nil {
						for _, env := range project.Environments {
							if strings.ToLower(env.Name) == "production" {
								projectCfg.EnvironmentUUID = env.UUID
								break
							}
						}
					}

					// Create if still missing
					if projectCfg.EnvironmentUUID == "" {
						prodEnv, err := client.CreateEnvironment(projectCfg.ProjectUUID, "production")
						if err != nil && !api.IsConflict(err) {
							return err
						}
						if prodEnv != nil {
							projectCfg.EnvironmentUUID = prodEnv.UUID
						}
					}

					return config.SaveProject(projectCfg)
				}
				return nil
			},
		})
	}

	// Create GitHub repo if needed
	if needsRepoCreation {
		tasks = append(tasks, ui.Task{
			Name:         "create-repo",
			ActiveName:   "Creating GitHub repository...",
			CompleteName: "✓ Created GitHub repository",
			Action: func() error {
				// Create README if it doesn't exist
				if err := createReadmeIfMissing(projectCfg); err != nil {
					// Ignore README creation errors
				}

				_, err := ghClient.CreateRepo(repoName, fmt.Sprintf("Deployment repository for %s", projectCfg.Name), isPrivate)
				if err != nil {
					return fmt.Errorf("failed to create GitHub repository %q: %w", repoName, err)
				}
				projectCfg.GitHubRepo = repoName
				return config.SaveProject(projectCfg)
			},
		})
	}

	// Initialize git if needed
	if !git.IsRepo(".") {
		tasks = append(tasks, ui.Task{
			Name:         "init-git",
			ActiveName:   "Initializing git repository...",
			CompleteName: "✓ Initialized git repository",
			Action: func() error {
				if err := git.Init("."); err != nil {
					return fmt.Errorf("failed to initialize git repository: %w", err)
				}
				return nil
			},
		})
	}

	// Push code to GitHub
	branch := projectCfg.Branch
	if branch == "" {
		b, _ := git.GetCurrentBranch(".")
		if b == "" {
			branch = config.DefaultBranch
		} else {
			branch = b
		}
	}

	tasks = append(tasks, ui.Task{
		Name:         "push-code",
		ActiveName:   "Pushing code to GitHub...",
		CompleteName: "✓ Pushed code to GitHub",
		Action: func() error {
			// Use HTTPS URL without embedded token (more secure)
			remoteURL := fmt.Sprintf("https://github.com/%s.git", fullRepoName)
			if err := git.SetRemote(".", "origin", remoteURL); err != nil {
				return fmt.Errorf("failed to configure git remote: %w", err)
			}

			if err := git.AutoCommitVerbose(".", verbose); err != nil {
				// Ignore if nothing to commit
			}

			// Use secure token-based authentication
			return git.PushWithTokenVerbose(".", "origin", branch, globalCfg.GitHubToken, verbose)
		},
	})

	// Create Coolify app if needed
	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		tasks = append(tasks, ui.Task{
			Name:         "create-app",
			ActiveName:   "Creating Coolify application...",
			CompleteName: "✓ Created Coolify application",
			Action: func() error {
				buildPack := projectCfg.BuildPack
				if buildPack == "" {
					buildPack = detect.BuildPackNixpacks
				}

				port := projectCfg.Port
				if port == "" {
					port = config.DefaultPort
				}

				// Use Coolify's static site feature for static builds
				isStatic := buildPack == detect.BuildPackStatic

				// Enable health check for static sites
				healthCheckEnabled := isStatic
				healthCheckPath := "/"

				resp, err := client.CreatePrivateGitHubApp(&api.CreatePrivateGitHubAppRequest{
					ProjectUUID:        projectCfg.ProjectUUID,
					ServerUUID:         projectCfg.ServerUUID,
					EnvironmentUUID:    projectCfg.EnvironmentUUID,
					GitHubAppUUID:      githubAppUUID,
					GitRepository:      fullRepoName,
					GitBranch:          branch,
					Name:               projectCfg.Name,
					BuildPack:          buildPack,
					IsStatic:           isStatic,
					Domains:            projectCfg.Domain,
					InstallCommand:     projectCfg.InstallCommand,
					BuildCommand:       projectCfg.BuildCommand,
					StartCommand:       projectCfg.StartCommand,
					PublishDirectory:   projectCfg.PublishDir,
					PortsExposes:       port,
					HealthCheckEnabled: healthCheckEnabled,
					HealthCheckPath:    healthCheckPath,
					InstantDeploy:      false,
				})
				if err != nil {
					return fmt.Errorf("failed to create Coolify application %q with GitHub integration: %w", projectCfg.Name, err)
				}
				appUUID = resp.UUID
				projectCfg.AppUUID = appUUID

				// Note: Preview deployments for Git apps are enabled by default in Coolify
				// when using GitHub App integration. No additional configuration needed.

				return config.SaveProject(projectCfg)
			},
		})
	}

	// Trigger deployment
	tasks = append(tasks, ui.Task{
		Name:         "trigger-deploy",
		ActiveName:   "Triggering deployment...",
		CompleteName: "✓ Triggered deployment",
		Action: func() error {
			_, err := client.Deploy(appUUID, false, prNumber)
			if err != nil {
				return fmt.Errorf("failed to trigger deployment: %w", err)
			}
			return nil
		},
	})

	// Run all tasks with verbose mode
	if err := ui.RunTasksVerbose(tasks, verbose); err != nil {
		ui.Spacer()
		ui.Error("Deployment setup failed")
		return err
	}

	// ===== PHASE 3: Watch deployment (keep logs as-is) =====
	
	ui.Spacer()
	ui.Info("Watching deployment...")

	success := watchDeployment(client, appUUID)

	ui.Spacer()

	if !success {
		ui.Error("Deployment failed")
		ui.Spacer()
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s logs' to view deployment logs", execName()),
			"Check the Coolify dashboard for more details",
		})
		return fmt.Errorf("deployment failed")
	}

	// Get app info for URL
	ui.Success("Deployment complete")
	ui.Spacer()

	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		ui.KeyValue("URL", ui.InfoStyle.Render(app.FQDN))
	}

	ui.Spacer()
	return nil
}

// watchDeployment polls the deployment status and displays build logs
// Returns true if deployment succeeded, false if it failed
func watchDeployment(client *api.Client, appUUID string) bool {
	ui.Spacer()

	debug := os.Getenv("CDP_DEBUG") != ""

	if debug {
		fmt.Printf("[DEBUG] Watching app UUID: %s\n", appUUID)
	}

	// State tracking
	type watchState struct {
		lastLogLen         int
		lastDeploymentUUID string
		hadDeployment      bool
		emptyCount         int
		consecutiveErrors  int
	}
	state := &watchState{}

	const (
		maxAttempts          = 120 // 4 minutes max (2s intervals)
		pollInterval         = 2 * time.Second
		maxEmptyChecks       = 3
		maxConsecutiveErrors = 5
		noDeploymentTimeout  = 15 // attempts
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Get deployments for the app
		deployments, err := client.ListDeployments(appUUID)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] ListDeployments error: %v\n", err)
			}
			state.consecutiveErrors++
			// Early exit if too many consecutive errors
			if state.consecutiveErrors >= maxConsecutiveErrors {
				if debug {
					fmt.Printf("[DEBUG] Too many consecutive errors, giving up\n")
				}
				return false
			}
			time.Sleep(pollInterval)
			continue
		}

		// Reset error counter on successful API call
		state.consecutiveErrors = 0

		if debug && attempt == 0 {
			fmt.Printf("[DEBUG] Found %d deployments\n", len(deployments))
			for i, d := range deployments {
				fmt.Printf("[DEBUG]   [%d] UUID=%s DeploymentUUID=%s Status=%s\n", i, d.UUID, d.DeploymentUUID, d.Status)
			}
		}

		if len(deployments) == 0 {
			state.emptyCount++
			// If we had a deployment but now it's gone, verify final status
			if state.hadDeployment && state.emptyCount >= maxEmptyChecks {
				// Check application status to determine final state
				app, err := client.GetApplication(appUUID)
				if err != nil {
					if debug {
						fmt.Printf("[DEBUG] GetApplication error: %v\n", err)
					}
					// If we can't check, wait a bit more
					time.Sleep(pollInterval)
					continue
				}
				appStatus := strings.ToLower(app.Status)
				if debug {
					fmt.Printf("[DEBUG] Application status: %s\n", appStatus)
				}
				// Determine success based on app status
				switch appStatus {
				case "running":
					return true
				case "exited", "error", "failed":
					return false
				default:
					// Other statuses - deployment may still be processing
					if state.emptyCount >= maxEmptyChecks+3 {
						// Waited long enough, assume success
						return true
					}
				}
			}
			// Early exit: if we never saw a deployment after reasonable wait
			if !state.hadDeployment && attempt >= noDeploymentTimeout {
				if debug {
					fmt.Printf("[DEBUG] No deployment found after %d attempts\n", attempt)
				}
				return false
			}
			if debug {
				fmt.Printf("[DEBUG] No deployments (attempt %d, hadDeployment=%v)\n", attempt, state.hadDeployment)
			}
			time.Sleep(pollInterval)
			continue
		}

		// We have deployments
		state.hadDeployment = true
		state.emptyCount = 0

		// Get the latest deployment
		latest := deployments[0]

		// Use DeploymentUUID if available, otherwise fall back to UUID
		deployUUID := latest.DeploymentUUID
		if deployUUID == "" {
			deployUUID = latest.UUID
		}

		if debug && deployUUID != state.lastDeploymentUUID {
			fmt.Printf("[DEBUG] Using deployment UUID: %s\n", deployUUID)
		}

		// If this is a new deployment, reset log position
		if deployUUID != state.lastDeploymentUUID {
			state.lastDeploymentUUID = deployUUID
			state.lastLogLen = 0
		}

		// Try to get full deployment details including logs
		detail, err := client.GetDeployment(deployUUID)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] GetDeployment error: %v\n", err)
			}
		} else {
			if debug && attempt%10 == 0 {
				fmt.Printf("[DEBUG] Detail: Status=%s RawLogLen=%d\n", detail.Status, len(detail.Logs))
			}

			// Parse the JSON logs to extract readable output
			parsedLogs := api.ParseLogs(detail.Logs)
			if len(parsedLogs) > state.lastLogLen {
				// Print new log content with dim styling
				newContent := parsedLogs[state.lastLogLen:]
				lines := strings.Split(newContent, "\n")
				for _, line := range lines {
					if line != "" {
						fmt.Println(ui.DimStyle.Render("  " + line))
					}
				}
				state.lastLogLen = len(parsedLogs)
			}

			// Check deployment status for early exit
			status := strings.ToLower(detail.Status)

			if status == "finished" {
				return true
			}
			if status == "failed" || status == "error" || status == "cancelled" {
				return false
			}
			// "running", "in_progress", "queued" etc. mean still deploying - keep waiting
		}

		// Fallback: check status from deployment list for early exit
		status := strings.ToLower(latest.Status)
		if status == "finished" {
			return true
		}
		if status == "failed" || status == "error" || status == "cancelled" {
			return false
		}

		time.Sleep(pollInterval)
	}

	// Timed out - check final application status one more time
	if debug {
		fmt.Printf("[DEBUG] Timeout reached, checking final app status\n")
	}
	app, err := client.GetApplication(appUUID)
	if err == nil && strings.ToLower(app.Status) == "running" {
		return true
	}

	return false
}

// createReadmeIfMissing creates a README.md file if one doesn't exist
func createReadmeIfMissing(cfg *config.ProjectConfig) error {
	readmePath := filepath.Join(".", "README.md")
	if _, err := os.Stat(readmePath); err == nil {
		return nil // README already exists
	}

	content := fmt.Sprintf(`# %s

## Framework

%s

## Deployment

This project is deployed to Coolify.
`, cfg.Name, cfg.Framework)

	return os.WriteFile(readmePath, []byte(content), 0644)
}

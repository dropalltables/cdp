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

By default, deploys to the preview environment.
Use --prod to deploy to production.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploy(prodFlag)
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(isProd bool) error {
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

	// First-time setup if no project config exists
	if projectCfg == nil {
		ui.Section("New Project Setup")
		ui.Dim("Let's configure your project for deployment")
		ui.Spacer()

		projectCfg, err = firstTimeSetup(client, globalCfg, isProd)
		if err != nil {
			return err
		}
	}

	env := config.EnvPreview
	if isProd {
		env = config.EnvProduction
	}

	// Show deployment summary
	ui.Section("Deploy")
	ui.KeyValue("Project", projectCfg.Name)
	ui.KeyValue("Environment", env)
	ui.KeyValue("Method", projectCfg.DeployMethod)
	ui.Spacer()

	// Deploy based on method
	if projectCfg.DeployMethod == config.DeployMethodDocker {
		return deployDocker(client, globalCfg, projectCfg, env)
	}
	return deployGit(client, globalCfg, projectCfg, env)
}

func firstTimeSetup(client *api.Client, globalCfg *config.GlobalConfig, isProd bool) (*config.ProjectConfig, error) {
	ui.StepProgress(1, 5, "Framework Detection")

	// Detect framework
	var framework *detect.FrameworkInfo
	err := ui.WithSpinner("Analyzing project", func() error {
		var err error
		framework, err = detect.Detect(".")
		return err
	})
	if err != nil {
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

	// Select server
	ui.Divider()
	ui.StepProgress(3, 5, "Server Selection")
	ui.Spacer()

	var servers []api.Server
	err = ui.WithSpinner("Loading servers", func() error {
		var err error
		servers, err = client.ListServers()
		return err
	})
	if err != nil {
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

	// Select or create project
	ui.Divider()
	ui.StepProgress(4, 5, "Project Configuration")
	ui.Spacer()

	var projects []api.Project
	err = ui.WithSpinner("Loading projects", func() error {
		var err error
		projects, err = client.ListProjects()
		return err
	})
	if err != nil {
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
	var previewEnvUUID, prodEnvUUID string

	if selectedProject == "+ Create new project" {
		// Create new project
		ui.Spacer()
		projectName := getWorkingDirName()
		name, err := ui.InputWithDefault("Project name:", projectName)
		if err != nil {
			return nil, err
		}

		ui.Spacer()
		err = ui.WithSpinner(fmt.Sprintf("Creating project '%s'", name), func() error {
			newProject, err := client.CreateProject(name, "Created by CDP")
			if err != nil {
				return err
			}
			projectUUID = newProject.UUID

			// Fetch project to check for auto-created environments
			project, err := client.GetProject(projectUUID)
			if err == nil {
				for _, env := range project.Environments {
					if strings.ToLower(env.Name) == "preview" {
						previewEnvUUID = env.UUID
					}
					if strings.ToLower(env.Name) == "production" {
						prodEnvUUID = env.UUID
					}
				}
			}

			// Create missing environments
			if previewEnvUUID == "" {
				previewEnv, err := client.CreateEnvironment(projectUUID, "preview")
				if err != nil {
					return fmt.Errorf("failed to create preview environment: %w", err)
				}
				previewEnvUUID = previewEnv.UUID
			}

			if prodEnvUUID == "" {
				prodEnv, err := client.CreateEnvironment(projectUUID, "production")
				if err != nil {
					return fmt.Errorf("failed to create production environment: %w", err)
				}
				prodEnvUUID = prodEnv.UUID
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Use existing project
		project := projectMap[selectedProject]
		projectUUID = project.UUID

		// Fetch fresh project data to get current environments
		freshProject, err := client.GetProject(projectUUID)
		if err == nil {
			project = *freshProject
		}

		// Find existing environments
		for _, env := range project.Environments {
			if strings.ToLower(env.Name) == "preview" {
				previewEnvUUID = env.UUID
			}
			if strings.ToLower(env.Name) == "production" {
				prodEnvUUID = env.UUID
			}
		}

		// Create missing environments (ignore 409 conflicts)
		if previewEnvUUID == "" {
			env, err := client.CreateEnvironment(projectUUID, "preview")
			if err != nil {
				// If 409 conflict, re-fetch project to get the existing env UUID
				if api.IsConflict(err) {
					if proj, err := client.GetProject(projectUUID); err == nil {
						for _, e := range proj.Environments {
							if strings.ToLower(e.Name) == "preview" {
								previewEnvUUID = e.UUID
								break
							}
						}
					}
				}
				if previewEnvUUID == "" {
					return nil, fmt.Errorf("failed to create preview environment: %w", err)
				}
			} else {
				previewEnvUUID = env.UUID
			}
		}
		if prodEnvUUID == "" {
			env, err := client.CreateEnvironment(projectUUID, "production")
			if err != nil {
				// If 409 conflict, re-fetch project to get the existing env UUID
				if api.IsConflict(err) {
					if proj, err := client.GetProject(projectUUID); err == nil {
						for _, e := range proj.Environments {
							if strings.ToLower(e.Name) == "production" {
								prodEnvUUID = e.UUID
								break
							}
						}
					}
				}
				if prodEnvUUID == "" {
					return nil, fmt.Errorf("failed to create production environment: %w", err)
				}
			} else {
				prodEnvUUID = env.UUID
			}
		}
	}

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

		// Platform (for Docker builds)
		if deployMethod == config.DeployMethodDocker {
			platformOptions := []string{"linux/amd64 (Intel/AMD)", "linux/arm64 (ARM)"}
			platformChoice, err := ui.Select("Target platform:", platformOptions)
			if err != nil {
				return nil, err
			}
			if strings.Contains(platformChoice, "arm64") {
				platform = "linux/arm64"
			}
		}

		// Branch (for Git deploys)
		if deployMethod == config.DeployMethodGit {
			branch, err = ui.InputWithDefault("Git branch:", branch)
			if err != nil {
				return nil, err
			}
		}

		// Domain
		useDomain, err := ui.Confirm("Configure custom domain?")
		if err != nil {
			return nil, err
		}
		if useDomain {
			domain, err = ui.Input("Domain:", "app.example.com")
			if err != nil {
				return nil, err
			}
		}
	}

	// Create project config
	projectCfg := &config.ProjectConfig{
		Name:           getWorkingDirName(),
		DeployMethod:   deployMethod,
		ProjectUUID:    projectUUID,
		ServerUUID:     serverUUID,
		PreviewEnvUUID: previewEnvUUID,
		ProdEnvUUID:    prodEnvUUID,
		AppUUIDs:       make(map[string]string),
		Framework:      framework.Name,
		BuildPack:      framework.BuildPack,
		InstallCommand: framework.InstallCommand,
		BuildCommand:   framework.BuildCommand,
		StartCommand:   framework.StartCommand,
		PublishDir:     framework.PublishDirectory,
		Port:           port,
		Platform:       platform,
		Branch:         branch,
		Domain:         domain,
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
	err = ui.WithSpinner("Saving configuration", func() error {
		return config.SaveProject(projectCfg)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

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
		ui.Info(fmt.Sprintf("Using %s deployment", options[0]))
		ui.Spacer()
		return optionMap[options[0]], nil
	}

	// Show options
	selected, err := ui.Select("Choose deployment method:", options)
	if err != nil {
		return "", err
	}
	return optionMap[selected], nil
}

func deployDocker(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, env string) error {
	// Generate tag
	tag := docker.GenerateTag(env)

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

	// Don't use spinner for Docker build - let Docker's progress display through
	ui.Info("Building Docker image...")
	ui.Spacer()
	err := docker.Build(&docker.BuildOptions{
		Dir:       ".",
		ImageName: projectCfg.DockerImage,
		Tag:       tag,
		Framework: framework,
		Platform:  projectCfg.Platform,
	})
	ui.Spacer()
	if err != nil {
		ui.Error("Build failed")
		return fmt.Errorf("build failed: %w", err)
	}
	ui.Success("Image built successfully")

	// Push image
	ui.Info("Pushing to registry...")
	ui.Spacer()
	err = docker.Push(&docker.PushOptions{
		ImageName: projectCfg.DockerImage,
		Tag:       tag,
		Registry:  globalCfg.DockerRegistry.URL,
		Username:  globalCfg.DockerRegistry.Username,
		Password:  globalCfg.DockerRegistry.Password,
	})
	ui.Spacer()
	if err != nil {
		ui.Error("Push failed")
		return fmt.Errorf("push failed: %w", err)
	}
	ui.Success("Image pushed successfully")

	// Create or update Coolify app
	ui.Spacer()
	ui.Divider()
	ui.Bold("Coolify Deployment")
	ui.Spacer()

	appUUID, exists := projectCfg.AppUUIDs[env]
	if !exists {
		// Create new app
		envUUID := projectCfg.PreviewEnvUUID
		if env == config.EnvProduction {
			envUUID = projectCfg.ProdEnvUUID
		}

		port := projectCfg.Port
		if port == "" {
			port = config.DefaultPort
		}

		err := ui.WithSpinner("Creating application", func() error {
			resp, err := client.CreateDockerImageApp(&api.CreateDockerImageAppRequest{
				ProjectUUID:             projectCfg.ProjectUUID,
				ServerUUID:              projectCfg.ServerUUID,
				EnvironmentUUID:         envUUID,
				Name:                    fmt.Sprintf("%s-%s", projectCfg.Name, env),
				DockerRegistryImageName: projectCfg.DockerImage,
				DockerRegistryImageTag:  tag,
				PortsExposes:            port,
				InstantDeploy:           true,
			})
			if err != nil {
				return err
			}
			appUUID = resp.UUID
			projectCfg.AppUUIDs[env] = appUUID
			return config.SaveProject(projectCfg)
		})
		if err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}
	} else {
		// Update existing app and trigger deploy
		err := ui.WithSpinner("Triggering deployment", func() error {
			err := client.UpdateApplication(appUUID, map[string]interface{}{
				"docker_registry_image_tag": tag,
			})
			if err != nil {
				return err
			}
			_, err = client.Deploy(appUUID, false)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to deploy: %w", err)
		}
	}

	// Watch deployment progress
	ui.Spacer()
	ui.Divider()
	ui.Bold("Deployment Progress")
	ui.Spacer()

	success := watchDeployment(client, appUUID)

	ui.Spacer()
	ui.Divider()

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

func deployGit(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, env string) error {
	ghClient := git.NewGitHubClient(globalCfg.GitHubToken)

	ui.Divider()
	ui.Bold("Git Deployment")
	ui.Spacer()

	// Get GitHub username
	var user *git.User
	err := ui.WithSpinner("Checking GitHub connection", func() error {
		var err error
		user, err = ghClient.GetUser()
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to connect to GitHub: %w", err)
	}

	repoName := projectCfg.GitHubRepo
	fullRepoName := fmt.Sprintf("%s/%s", user.Login, repoName)

	// Check if repo exists, create if not
	if !ghClient.RepoExists(user.Login, repoName) {
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

		// Ask for visibility
		visibilityOptions := []string{"Private", "Public"}
		visibility, err := ui.Select("Repository visibility:", visibilityOptions)
		if err != nil {
			return err
		}
		isPrivate := visibility == "Private"
		projectCfg.GitHubPrivate = isPrivate

		// Show confirmation
		ui.Spacer()
		ui.KeyValue("Repository", fullRepoName)
		ui.KeyValue("Visibility", visibility)
		ui.KeyValue("Branch", "main")
		ui.Spacer()

		proceed, err := ui.Confirm("Create repository?")
		if err != nil {
			return err
		}
		if !proceed {
			ui.Dim("Cancelled")
			return fmt.Errorf("cancelled")
		}

		// Create README if it doesn't exist
		ui.Spacer()
		if err := createReadmeIfMissing(projectCfg); err != nil {
			ui.Dim(fmt.Sprintf("Could not create README: %v", err))
		}

		// Create the repo
		err = ui.WithSpinner("Creating GitHub repository", func() error {
			_, err := ghClient.CreateRepo(repoName, fmt.Sprintf("Deployment repository for %s", projectCfg.Name), isPrivate)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to create repository: %w", err)
		}

		// Save updated config
		config.SaveProject(projectCfg)
	}

	// Initialize git if needed
	ui.Spacer()
	ui.Divider()
	ui.Bold("Pushing Code")
	ui.Spacer()

	if !git.IsRepo(".") {
		err := ui.WithSpinner("Initializing git repository", func() error {
			return git.Init(".")
		})
		if err != nil {
			return fmt.Errorf("failed to initialize git: %w", err)
		}
	}

	// Set remote
	remoteURL := fmt.Sprintf("https://%s@github.com/%s.git", globalCfg.GitHubToken, fullRepoName)
	if err := git.SetRemote(".", "origin", remoteURL); err != nil {
		return fmt.Errorf("failed to set remote: %w", err)
	}

	// Commit and push
	branch := projectCfg.Branch
	if branch == "" {
		branch, _ = git.GetCurrentBranch(".")
		if branch == "" {
			branch = config.DefaultBranch
		}
	}

	err = ui.WithSpinner(fmt.Sprintf("Pushing to %s", fullRepoName), func() error {
		if err := git.AutoCommit("."); err != nil {
			// Ignore if nothing to commit
		}
		return git.Push(".", "origin", branch)
	})
	if err != nil {
		return fmt.Errorf("failed to push code: %w", err)
	}

	// Get GitHub App for Coolify to use as git source
	ui.Spacer()
	ui.Divider()
	ui.Bold("Coolify Configuration")
	ui.Spacer()

	var githubApps []api.GitHubApp
	err = ui.WithSpinner("Loading GitHub Apps", func() error {
		var err error
		githubApps, err = client.ListGitHubApps()
		return err
	})
	if err != nil {
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
	if len(githubApps) == 1 {
		githubAppUUID = githubApps[0].UUID
		ui.Spacer()
		ui.Info(fmt.Sprintf("Using GitHub App: %s", githubApps[0].Name))
		ui.Spacer()
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
		ui.Spacer()
	}

	// Create or deploy Coolify app
	appUUID, exists := projectCfg.AppUUIDs[env]
	if !exists {
		envUUID := projectCfg.PreviewEnvUUID
		if env == config.EnvProduction {
			envUUID = projectCfg.ProdEnvUUID
		}

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

		err := ui.WithSpinner("Creating Coolify application", func() error {
			resp, err := client.CreatePrivateGitHubApp(&api.CreatePrivateGitHubAppRequest{
				ProjectUUID:        projectCfg.ProjectUUID,
				ServerUUID:         projectCfg.ServerUUID,
				EnvironmentUUID:    envUUID,
				GitHubAppUUID:      githubAppUUID,
				GitRepository:      fullRepoName,
				GitBranch:          branch,
				Name:               fmt.Sprintf("%s-%s", projectCfg.Name, env),
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
				InstantDeploy:      true,
			})
			if err != nil {
				return err
			}
			appUUID = resp.UUID
			projectCfg.AppUUIDs[env] = appUUID
			return config.SaveProject(projectCfg)
		})
		if err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}
	} else {
		// Trigger deploy for existing app
		err := ui.WithSpinner("Triggering deployment", func() error {
			_, err := client.Deploy(appUUID, false)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to trigger deployment: %w", err)
		}
	}

	// Watch deployment progress
	ui.Spacer()
	ui.Divider()
	ui.Bold("Deployment Progress")
	ui.Spacer()

	success := watchDeployment(client, appUUID)

	ui.Spacer()
	ui.Divider()

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
	ui.Dim("Build logs:")
	ui.Divider()

	debug := os.Getenv("CDP_DEBUG") != ""

	if debug {
		fmt.Printf("[DEBUG] Watching app UUID: %s\n", appUUID)
	}

	lastLogLen := 0
	maxAttempts := 120 // 4 minutes max (2s intervals)
	attempt := 0
	var lastDeploymentUUID string
	hadDeployment := false
	emptyCount := 0

	for attempt < maxAttempts {
		// Get deployments for the app
		deployments, err := client.ListDeployments(appUUID)
		if err != nil {
			if debug {
				fmt.Printf("[DEBUG] ListDeployments error: %v\n", err)
			}
			time.Sleep(2 * time.Second)
			attempt++
			continue
		}

		if debug && attempt == 0 {
			fmt.Printf("[DEBUG] Found %d deployments\n", len(deployments))
			for i, d := range deployments {
				fmt.Printf("[DEBUG]   [%d] UUID=%s DeploymentUUID=%s Status=%s\n", i, d.UUID, d.DeploymentUUID, d.Status)
			}
		}

		if len(deployments) == 0 {
			emptyCount++
			// If we had a deployment but now it's gone, check application status
			if hadDeployment && emptyCount >= 3 {
				// Check application status to determine if deployment succeeded
				app, err := client.GetApplication(appUUID)
				if err != nil {
					if debug {
						fmt.Printf("[DEBUG] GetApplication error: %v\n", err)
					}
					return true // Assume success if we can't check
				}
				appStatus := strings.ToLower(app.Status)
				if debug {
					fmt.Printf("[DEBUG] Application status: %s\n", appStatus)
				}
				// "running" means success, other statuses may indicate issues
				if appStatus == "running" {
					return true
				} else if appStatus == "exited" || appStatus == "error" || appStatus == "failed" {
					return false
				}
				// Other statuses (starting, etc.) - assume success
				return true
			}
			// If we never saw a deployment and waited a while, give up
			if !hadDeployment && attempt >= 15 {
				if debug {
					fmt.Printf("[DEBUG] No deployment found after %d attempts\n", attempt)
				}
				return false
			}
			if debug {
				fmt.Printf("[DEBUG] No deployments (attempt %d, hadDeployment=%v)\n", attempt, hadDeployment)
			}
			time.Sleep(2 * time.Second)
			attempt++
			continue
		}

		// We have deployments
		hadDeployment = true
		emptyCount = 0

		// Get the latest deployment
		latest := deployments[0]

		// Use DeploymentUUID if available, otherwise fall back to UUID
		deployUUID := latest.DeploymentUUID
		if deployUUID == "" {
			deployUUID = latest.UUID
		}

		if debug && deployUUID != lastDeploymentUUID {
			fmt.Printf("[DEBUG] Using deployment UUID: %s\n", deployUUID)
		}

		// If this is a new deployment, reset log position
		if deployUUID != lastDeploymentUUID {
			lastDeploymentUUID = deployUUID
			lastLogLen = 0
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
			if len(parsedLogs) > lastLogLen {
				// Print new log content
				newContent := parsedLogs[lastLogLen:]
				fmt.Print(newContent)
				if !strings.HasSuffix(newContent, "\n") {
					fmt.Println()
				}
				lastLogLen = len(parsedLogs)
			}

			// Check deployment status
			status := strings.ToLower(detail.Status)

			if status == "finished" {
				return true
			}
			if status == "failed" || status == "error" || status == "cancelled" {
				return false
			}
			// "running", "in_progress", "queued" etc. mean still deploying - keep waiting
		}

		// Fallback: check status from deployment list
		status := strings.ToLower(latest.Status)
		if status == "finished" {
			return true
		}
		if status == "failed" || status == "error" || status == "cancelled" {
			return false
		}

		time.Sleep(2 * time.Second)
		attempt++
	}

	// Timed out
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

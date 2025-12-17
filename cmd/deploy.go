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

	// Migrate legacy config if needed
	if projectCfg.AppUUID == "" && len(projectCfg.AppUUIDs) > 0 {
		if err := migrateLegacyConfig(projectCfg); err != nil {
			ui.Warning("Failed to migrate config, please redeploy")
			return fmt.Errorf("config migration failed: %w", err)
		}
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

// migrateLegacyConfig migrates old two-app config to new single-app config
func migrateLegacyConfig(cfg *config.ProjectConfig) error {
	// Use production app as the main app
	if prodUUID, ok := cfg.AppUUIDs[config.EnvProduction]; ok && prodUUID != "" {
		cfg.AppUUID = prodUUID
		cfg.EnvironmentUUID = cfg.ProdEnvUUID
	} else if previewUUID, ok := cfg.AppUUIDs[config.EnvPreview]; ok && previewUUID != "" {
		cfg.AppUUID = previewUUID
		cfg.EnvironmentUUID = cfg.PreviewEnvUUID
	}
	
	// Clear legacy fields
	cfg.AppUUIDs = nil
	cfg.PreviewEnvUUID = ""
	cfg.ProdEnvUUID = ""
	
	return config.SaveProject(cfg)
}

func firstTimeSetup(client *api.Client, globalCfg *config.GlobalConfig) (*config.ProjectConfig, error) {
	ui.StepProgress(1, 5, "Framework Detection")

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

	// Select server
	ui.Divider()
	ui.StepProgress(3, 5, "Server Selection")
	ui.Spacer()

	spinner := ui.NewSpinner()
	spinner.Start()
	spinner.UpdateMessage("Loading servers...")
	
	servers, err := client.ListServers()
	if err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to load servers")
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	spinner.Complete("✓ Loaded servers")

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

	// Select or create project (questions only)
	ui.Divider()
	ui.StepProgress(4, 5, "Project Configuration")
	ui.Spacer()

	spinner = ui.NewSpinner()
	spinner.Start()
	spinner.UpdateMessage("Loading projects...")
	
	projects, err := client.ListProjects()
	if err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to load projects")
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	spinner.Complete("✓ Loaded projects")

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
	needsProjectCreation := selectedProject == "+ Create new project"

	if needsProjectCreation {
		// Ask for project name
		ui.Spacer()
		projectName := getWorkingDirName()
		newProjectName, err = ui.InputWithDefault("Project name:", projectName)
		if err != nil {
			return nil, err
		}
	} else {
		// Use existing project
		project := projectMap[selectedProject]
		projectUUID = project.UUID
		
		// Will check/create environment at the end with other API calls
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

	// Build image (keep Docker build output visible - no spinner)
	framework := &detect.FrameworkInfo{
		Name:             projectCfg.Framework,
		InstallCommand:   projectCfg.InstallCommand,
		BuildCommand:     projectCfg.BuildCommand,
		StartCommand:     projectCfg.StartCommand,
		PublishDirectory: projectCfg.PublishDir,
	}

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

	// ===== API Operations with spinner =====
	
	ui.Spacer()
	ui.Divider()
	ui.Spacer()
	
	spinner := ui.NewSpinner()
	spinner.Start()
	
	// Create project and environment if needed
	if needsProjectCreation {
		spinner.UpdateMessage("Creating Coolify project...")
		newProject, err := client.CreateProject(projectCfg.Name, "Created by CDP")
		if err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to create project")
			return fmt.Errorf("failed to create project: %w", err)
		}
		projectCfg.ProjectUUID = newProject.UUID
		spinner.Complete("✓ Created Coolify project")

		// Fetch project to check for auto-created environments
		spinner.UpdateMessage("Setting up environment...")
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
				spinner.Stop()
				ui.Spacer()
				ui.Error("Failed to create production environment")
				return fmt.Errorf("failed to create production environment: %w", err)
			}
			projectCfg.EnvironmentUUID = prodEnv.UUID
		}
		spinner.Complete("✓ Set up environment")
		
		config.SaveProject(projectCfg)
	} else {
		// Check if environment exists for existing project
		spinner.UpdateMessage("Checking environment...")
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
					spinner.Stop()
					ui.Spacer()
					ui.Error("Failed to create production environment")
					return fmt.Errorf("failed to create production environment: %w", err)
				}
				if prodEnv != nil {
					projectCfg.EnvironmentUUID = prodEnv.UUID
				}
			}
			
			config.SaveProject(projectCfg)
		}
		spinner.Complete("✓ Environment ready")
	}

	// Push image
	spinner.UpdateMessage("Pushing image to registry...")
	err = docker.Push(&docker.PushOptions{
		ImageName: projectCfg.DockerImage,
		Tag:       tag,
		Registry:  globalCfg.DockerRegistry.URL,
		Username:  globalCfg.DockerRegistry.Username,
		Password:  globalCfg.DockerRegistry.Password,
	})
	if err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to push image")
		return fmt.Errorf("push failed: %w", err)
	}
	spinner.Complete("✓ Pushed image to registry")

	// Create Coolify app if needed
	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		spinner.UpdateMessage("Creating Coolify application...")
		
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
			PortsExposes:  port,
			InstantDeploy: false, // We'll deploy manually with PR parameter
		})
		if err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to create Coolify application")
			return fmt.Errorf("failed to create application: %w", err)
		}
		appUUID = resp.UUID
		projectCfg.AppUUID = appUUID
		
		if err := config.SaveProject(projectCfg); err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to save configuration")
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		spinner.Complete("✓ Created Coolify application")
	}
	
	// Update image tag and trigger deployment
	spinner.UpdateMessage("Triggering deployment...")
	if err := client.UpdateApplication(appUUID, map[string]interface{}{
		"docker_registry_image_tag": tag,
	}); err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to update application")
		return fmt.Errorf("failed to update application: %w", err)
	}
	
	// Deploy with PR number (0 = production, >0 = preview)
	if _, err := client.Deploy(appUUID, false, prNumber); err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to trigger deployment")
		return fmt.Errorf("failed to deploy: %w", err)
	}
	spinner.Complete("✓ Triggered deployment")

	spinner.Stop()

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

	// ===== PHASE 1: Questions =====
	
	// Create project/environment if needed (spinner at end)
	needsProjectCreation := projectCfg.ProjectUUID == ""
	
	// Get GitHub username
	spinner := ui.NewSpinner()
	spinner.Start()
	spinner.UpdateMessage("Checking GitHub connection...")
	user, err := ghClient.GetUser()
	if err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to connect to GitHub")
		return fmt.Errorf("failed to connect to GitHub: %w", err)
	}
	spinner.Complete("✓ Connected to GitHub")

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

		// Ask for visibility
		visibilityOptions := []string{"Private", "Public"}
		visibility, err := ui.Select("Repository visibility:", visibilityOptions)
		if err != nil {
			return err
		}
		isPrivate = visibility == "Private"
		projectCfg.GitHubPrivate = isPrivate
	}

	// Get GitHub App for Coolify (question if multiple)
	ui.Spacer()
	spinner = ui.NewSpinner()
	spinner.Start()
	spinner.UpdateMessage("Loading GitHub Apps...")
	githubApps, err := client.ListGitHubApps()
	if err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to load GitHub Apps")
		ui.Spacer()
		ui.Dim("Configure a GitHub App in Coolify: Sources → GitHub App")
		return fmt.Errorf("failed to list GitHub Apps: %w", err)
	}
	spinner.Complete("✓ Loaded GitHub Apps")
	
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
	}

	// ===== PHASE 2: API Operations with spinner =====
	
	ui.Spacer()
	ui.Divider()
	ui.Spacer()
	
	spinner = ui.NewSpinner()
	spinner.Start()

	// Create project and environment if needed
	if needsProjectCreation {
		spinner.UpdateMessage("Creating Coolify project...")
		newProject, err := client.CreateProject(projectCfg.Name, "Created by CDP")
		if err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to create project")
			return fmt.Errorf("failed to create project: %w", err)
		}
		projectCfg.ProjectUUID = newProject.UUID
		spinner.Complete("✓ Created Coolify project")

		// Fetch project to check for auto-created environments
		spinner.UpdateMessage("Setting up environment...")
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
				spinner.Stop()
				ui.Spacer()
				ui.Error("Failed to create production environment")
				return fmt.Errorf("failed to create production environment: %w", err)
			}
			projectCfg.EnvironmentUUID = prodEnv.UUID
		}
		spinner.Complete("✓ Set up environment")
		
		config.SaveProject(projectCfg)
	} else {
		// Check if environment exists for existing project
		spinner.UpdateMessage("Checking environment...")
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
					spinner.Stop()
					ui.Spacer()
					ui.Error("Failed to create production environment")
					return fmt.Errorf("failed to create production environment: %w", err)
				}
				if prodEnv != nil {
					projectCfg.EnvironmentUUID = prodEnv.UUID
				}
			}
			
			config.SaveProject(projectCfg)
		}
		spinner.Complete("✓ Environment ready")
	}

	// Create GitHub repo if needed
	if needsRepoCreation {
		spinner.UpdateMessage("Creating GitHub repository...")
		
		// Create README if it doesn't exist
		if err := createReadmeIfMissing(projectCfg); err != nil {
			// Ignore README creation errors
		}
		
		_, err = ghClient.CreateRepo(repoName, fmt.Sprintf("Deployment repository for %s", projectCfg.Name), isPrivate)
		if err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to create GitHub repository")
			return fmt.Errorf("failed to create repository: %w", err)
		}
		projectCfg.GitHubRepo = repoName
		config.SaveProject(projectCfg)
		spinner.Complete("✓ Created GitHub repository")
	}

	// Initialize git if needed
	if !git.IsRepo(".") {
		spinner.UpdateMessage("Initializing git repository...")
		if err := git.Init("."); err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to initialize git")
			return fmt.Errorf("failed to initialize git: %w", err)
		}
		spinner.Complete("✓ Initialized git repository")
	}

	// Set remote and push
	spinner.UpdateMessage("Pushing code to GitHub...")
	
	remoteURL := fmt.Sprintf("https://%s@github.com/%s.git", globalCfg.GitHubToken, fullRepoName)
	if err := git.SetRemote(".", "origin", remoteURL); err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to set git remote")
		return fmt.Errorf("failed to set remote: %w", err)
	}

	branch := projectCfg.Branch
	if branch == "" {
		branch, _ = git.GetCurrentBranch(".")
		if branch == "" {
			branch = config.DefaultBranch
		}
	}

	if err := git.AutoCommit("."); err != nil {
		// Ignore if nothing to commit
	}
	
	if err := git.Push(".", "origin", branch); err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to push code")
		return fmt.Errorf("failed to push code: %w", err)
	}
	spinner.Complete("✓ Pushed code to GitHub")

	// Create Coolify app if needed
	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		spinner.UpdateMessage("Creating Coolify application...")
		
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
			InstantDeploy:      false, // We'll deploy manually with PR parameter
		})
		if err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to create Coolify application")
			return fmt.Errorf("failed to create application: %w", err)
		}
		appUUID = resp.UUID
		projectCfg.AppUUID = appUUID
		
		if err := config.SaveProject(projectCfg); err != nil {
			spinner.Stop()
			ui.Spacer()
			ui.Error("Failed to save configuration")
			return fmt.Errorf("failed to save configuration: %w", err)
		}
		spinner.Complete("✓ Created Coolify application")
	}
	
	// Trigger deployment
	spinner.UpdateMessage("Triggering deployment...")
	if _, err := client.Deploy(appUUID, false, prNumber); err != nil {
		spinner.Stop()
		ui.Spacer()
		ui.Error("Failed to trigger deployment")
		return fmt.Errorf("failed to trigger deployment: %w", err)
	}
	spinner.Complete("✓ Triggered deployment")

	spinner.Stop()

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
				// Print new log content with dim styling
				newContent := parsedLogs[lastLogLen:]
				lines := strings.Split(newContent, "\n")
				for _, line := range lines {
					if line != "" {
						fmt.Println(ui.DimStyle.Render("  " + line))
					}
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

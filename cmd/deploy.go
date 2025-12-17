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
	// Show welcome screen before anything else
	if err := ui.WelcomeScreen(); err != nil {
		return err
	}

	// Check login
	if err := checkLogin(); err != nil {
		return err
	}

	// Load global config
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load or create project config
	projectCfg, err := config.LoadProject()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Create API client
	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)

	// First-time setup if no project config exists
	if projectCfg == nil {
		projectCfg, err = firstTimeSetup(client, globalCfg, isProd)
		if err != nil {
			return err
		}
	}

	// Deploy based on method
	env := "preview"
	if isProd {
		env = "production"
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("Ready to deploy to %s environment", env))

	// Prompt to proceed or cancel
	if err := ui.ProceedOrCancel("Press Enter to proceed, Ctrl+C to cancel"); err != nil {
		return err
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("Deploying to %s environment...", env))

	if projectCfg.DeployMethod == "docker" {
		return deployDocker(client, globalCfg, projectCfg, env)
	}
	return deployGit(client, globalCfg, projectCfg, env)
}

func firstTimeSetup(client *api.Client, globalCfg *config.GlobalConfig, isProd bool) (*config.ProjectConfig, error) {
	fmt.Println("Setting up new project...")
	fmt.Println()

	// Detect framework
	spinner := ui.NewSpinner("Detecting framework...")
	spinner.Start()
	framework, err := detect.Detect(".")
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to detect framework: %w", err)
	}

	fmt.Printf("Detected: %s\n", framework.Name)
	fmt.Println()

	// Display and allow editing of build settings
	fmt.Println("Build Settings:")
	if framework.InstallCommand != "" {
		fmt.Printf("  Install command: %s\n", framework.InstallCommand)
	}
	if framework.BuildCommand != "" {
		fmt.Printf("  Build command:   %s\n", framework.BuildCommand)
	}
	if framework.StartCommand != "" {
		fmt.Printf("  Start command:   %s\n", framework.StartCommand)
	}
	if framework.PublishDirectory != "" {
		fmt.Printf("  Publish dir:     %s\n", framework.PublishDirectory)
	}
	fmt.Println()

	editSettings, err := ui.Confirm("Edit these settings?")
	if err != nil {
		return nil, err
	}
	if editSettings {
		framework, err = editBuildSettings(framework)
		if err != nil {
			return nil, err
		}
	}

	// Choose deployment method
	fmt.Println()
	deployMethod, err := chooseDeployMethod(globalCfg)
	if err != nil {
		return nil, err
	}

	// Select server
	fmt.Println()
	spinner = ui.NewSpinner("Loading servers...")
	spinner.Start()
	servers, err := client.ListServers()
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers found in Coolify. Please add a server first")
	}

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
	fmt.Println()
	spinner = ui.NewSpinner("Loading projects...")
	spinner.Start()
	projects, err := client.ListProjects()
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	projectOptions := make([]string, 0, len(projects)+1)
	projectOptions = append(projectOptions, "+ Create new project")
	projectMap := make(map[string]api.Project)
	for _, p := range projects {
		projectOptions = append(projectOptions, p.Name)
		projectMap[p.Name] = p
	}

	selectedProject, err := ui.Select("Select project:", projectOptions)
	if err != nil {
		return nil, err
	}

	var projectUUID string
	var previewEnvUUID, prodEnvUUID string

	if selectedProject == "+ Create new project" {
		// Create new project
		projectName := getWorkingDirName()
		name, err := ui.InputWithDefault("Project name:", projectName)
		if err != nil {
			return nil, err
		}

		spinner = ui.NewSpinner("Creating project...")
		spinner.Start()
		newProject, err := client.CreateProject(name, "Created by cdp")
		spinner.Stop()
		if err != nil {
			return nil, fmt.Errorf("failed to create project: %w", err)
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
		spinner = ui.NewSpinner("Creating environments...")
		spinner.Start()
		if previewEnvUUID == "" {
			previewEnv, err := client.CreateEnvironment(projectUUID, "preview")
			if err != nil {
				spinner.Stop()
				return nil, fmt.Errorf("failed to create preview environment: %w", err)
			}
			previewEnvUUID = previewEnv.UUID
		}

		if prodEnvUUID == "" {
			prodEnv, err := client.CreateEnvironment(projectUUID, "production")
			if err != nil {
				spinner.Stop()
				return nil, fmt.Errorf("failed to create production environment: %w", err)
			}
			prodEnvUUID = prodEnv.UUID
		}
		spinner.Stop()
		ui.Success("Project and environments created")
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
	fmt.Println()
	configureAdvanced, err := ui.Confirm("Configure advanced options (port, platform, domain, branch)?")
	if err != nil {
		return nil, err
	}

	port := framework.Port
	if port == "" {
		port = "3000"
	}
	platform := "linux/amd64"
	branch := "main"
	domain := "" // empty means Coolify auto-generates

	if configureAdvanced {
		fmt.Println()
		// Port
		port, err = ui.InputWithDefault("Port:", port)
		if err != nil {
			return nil, err
		}

		// Platform (for Docker builds)
		if deployMethod == "docker" {
			platformOptions := []string{"linux/amd64 (Intel/AMD)", "linux/arm64 (Apple Silicon/ARM)"}
			platformChoice, err := ui.Select("Target platform:", platformOptions)
			if err != nil {
				return nil, err
			}
			if strings.Contains(platformChoice, "arm64") {
				platform = "linux/arm64"
			}
		}

		// Branch (for Git deploys)
		if deployMethod == "git" {
			branch, err = ui.InputWithDefault("Git branch:", branch)
			if err != nil {
				return nil, err
			}
		}

		// Domain
		domainOptions := []string{"Auto-generate (Coolify wildcard)", "Custom domain"}
		domainChoice, err := ui.Select("Domain:", domainOptions)
		if err != nil {
			return nil, err
		}
		if domainChoice == "Custom domain" {
			domain, err = ui.Input("Custom domain:", "app.example.com")
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
	if deployMethod == "docker" {
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
	if err := config.SaveProject(projectCfg); err != nil {
		return nil, fmt.Errorf("failed to save project config: %w", err)
	}
	ui.Success("Created cdp.json")

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
		options = append(options, "Git-based (auto-manage GitHub repo)")
		optionMap["Git-based (auto-manage GitHub repo)"] = "git"
	}
	if hasDocker {
		options = append(options, "Docker-based (build & push image)")
		optionMap["Docker-based (build & push image)"] = "docker"
	}

	if len(options) == 0 {
		fmt.Println()
		ui.Warn("No deployment method available!")
		fmt.Println("Please configure at least one of:")
		fmt.Printf("  - GitHub token (run '%s login' and set up GitHub)\n", execName())
		fmt.Printf("  - Docker registry (run '%s login' and set up Docker)\n", execName())
		return "", fmt.Errorf("no deployment method configured")
	}

	// Always show available options
	selected, err := ui.Select("Deployment method:", options)
	if err != nil {
		return "", err
	}
	return optionMap[selected], nil
}

func deployDocker(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, env string) error {
	// Generate tag
	tag := docker.GenerateTag(env)

	// Build image
	fmt.Println()
	spinner := ui.NewSpinner("Building Docker image...")
	spinner.Start()

	framework := &detect.FrameworkInfo{
		Name:             projectCfg.Framework,
		InstallCommand:   projectCfg.InstallCommand,
		BuildCommand:     projectCfg.BuildCommand,
		StartCommand:     projectCfg.StartCommand,
		PublishDirectory: projectCfg.PublishDir,
	}

	err := docker.Build(&docker.BuildOptions{
		Dir:       ".",
		ImageName: projectCfg.DockerImage,
		Tag:       tag,
		Framework: framework,
		Platform:  projectCfg.Platform,
	})
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	ui.Success("Image built")

	// Push image
	fmt.Println()
	spinner = ui.NewSpinner("Pushing to registry...")
	spinner.Start()
	err = docker.Push(&docker.PushOptions{
		ImageName: projectCfg.DockerImage,
		Tag:       tag,
		Registry:  globalCfg.DockerRegistry.URL,
		Username:  globalCfg.DockerRegistry.Username,
		Password:  globalCfg.DockerRegistry.Password,
	})
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}
	ui.Success("Image pushed")

	// Create or update Coolify app
	appUUID, exists := projectCfg.AppUUIDs[env]
	if !exists {
		// Create new app
		fmt.Println()
		spinner = ui.NewSpinner("Creating Coolify application...")
		spinner.Start()

		envUUID := projectCfg.PreviewEnvUUID
		if env == "production" {
			envUUID = projectCfg.ProdEnvUUID
		}

		port := projectCfg.Port
		if port == "" {
			port = "3000"
		}

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
		spinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to create app: %w", err)
		}
		appUUID = resp.UUID
		projectCfg.AppUUIDs[env] = appUUID
		config.SaveProject(projectCfg)
		ui.Success("Application created and deploying")
	} else {
		// Update existing app and trigger deploy
		fmt.Println()
		spinner = ui.NewSpinner("Updating application...")
		spinner.Start()

		err := client.UpdateApplication(appUUID, map[string]interface{}{
			"docker_registry_image_tag": tag,
		})
		if err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to update app: %w", err)
		}

		_, err = client.Deploy(appUUID, false)
		spinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to trigger deploy: %w", err)
		}
		ui.Success("Deployment triggered")
	}

	// Watch deployment progress
	success := watchDeployment(client, appUUID)

	// Get app info for URL
	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		fmt.Println()
		fmt.Printf("Deployed to: %s\n", app.FQDN)
	}

	if !success {
		return fmt.Errorf("deployment failed")
	}
	return nil
}

func deployGit(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, env string) error {
	ghClient := git.NewGitHubClient(globalCfg.GitHubToken)

	// Get GitHub username
	user, err := ghClient.GetUser()
	if err != nil {
		return fmt.Errorf("failed to get GitHub user: %w", err)
	}

	repoName := projectCfg.GitHubRepo
	fullRepoName := fmt.Sprintf("%s/%s", user.Login, repoName)

	// Check if repo exists, create if not
	if !ghClient.RepoExists(user.Login, repoName) {
		fmt.Println()
		fmt.Println("GitHub Repository Setup:")
		fmt.Println(strings.Repeat("-", 40))

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
		fmt.Println()
		fmt.Println("Repository details:")
		fmt.Printf("  Name:       %s\n", fullRepoName)
		fmt.Printf("  Visibility: %s\n", visibility)
		fmt.Printf("  Branch:     main\n")
		fmt.Println()

		proceed, err := ui.Confirm("Create this repository?")
		if err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("cancelled by user")
		}

		// Create README if it doesn't exist
		if err := createReadmeIfMissing(projectCfg); err != nil {
			ui.Warn(fmt.Sprintf("Could not create README: %v", err))
		}

		// Create the repo
		spinner := ui.NewSpinner("Creating GitHub repository...")
		spinner.Start()
		_, err = ghClient.CreateRepo(repoName, fmt.Sprintf("Deployment repo for %s", projectCfg.Name), isPrivate)
		spinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to create repo: %w", err)
		}
		ui.Success(fmt.Sprintf("Created repository: %s", fullRepoName))

		// Save updated config
		config.SaveProject(projectCfg)
	}

	// Initialize git if needed
	if !git.IsRepo(".") {
		spinner := ui.NewSpinner("Initializing git...")
		spinner.Start()
		if err := git.Init("."); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to init git: %w", err)
		}
		spinner.Stop()
	}

	// Set remote
	remoteURL := fmt.Sprintf("https://%s@github.com/%s.git", globalCfg.GitHubToken, fullRepoName)
	if err := git.SetRemote(".", "origin", remoteURL); err != nil {
		return fmt.Errorf("failed to set remote: %w", err)
	}

	// Commit and push
	fmt.Println()
	spinner := ui.NewSpinner("Committing and pushing...")
	spinner.Start()
	if err := git.AutoCommit("."); err != nil {
		// Ignore if nothing to commit
	}
	// Use configured branch or current branch
	branch := projectCfg.Branch
	if branch == "" {
		branch, _ = git.GetCurrentBranch(".")
		if branch == "" {
			branch = "main"
		}
	}
	if err := git.Push(".", "origin", branch); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to push: %w", err)
	}
	spinner.Stop()
	ui.Success("Code pushed to GitHub")

	// Get GitHub App for Coolify to use as git source
	githubApps, err := client.ListGitHubApps()
	if err != nil {
		return fmt.Errorf("failed to list GitHub Apps: %w (configure a GitHub App in Coolify Sources)", err)
	}
	if len(githubApps) == 0 {
		return fmt.Errorf("no GitHub Apps configured in Coolify - add one in Sources > GitHub App")
	}

	// Select GitHub App (prompt if multiple)
	var githubAppUUID string
	if len(githubApps) == 1 {
		githubAppUUID = githubApps[0].UUID
	} else {
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

	// Create or deploy Coolify app
	appUUID, exists := projectCfg.AppUUIDs[env]
	if !exists {
		fmt.Println()
		spinner = ui.NewSpinner("Creating Coolify application...")
		spinner.Start()

		envUUID := projectCfg.PreviewEnvUUID
		if env == "production" {
			envUUID = projectCfg.ProdEnvUUID
		}

		buildPack := projectCfg.BuildPack
		if buildPack == "" {
			buildPack = "nixpacks"
		}

		port := projectCfg.Port
		if port == "" {
			port = "3000"
		}

		// Use Coolify's static site feature for static builds
		isStatic := buildPack == "static"

		// Enable health check for static sites
		healthCheckEnabled := isStatic
		healthCheckPath := "/"

		resp, err := client.CreatePrivateGitHubApp(&api.CreatePrivateGitHubAppRequest{
			ProjectUUID:        projectCfg.ProjectUUID,
			ServerUUID:         projectCfg.ServerUUID,
			EnvironmentUUID:    envUUID,
			GitHubAppUUID:      githubAppUUID,
			GitRepository:      fullRepoName, // Just "owner/repo" format for GitHub App
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
		spinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to create app: %w", err)
		}
		appUUID = resp.UUID
		projectCfg.AppUUIDs[env] = appUUID
		config.SaveProject(projectCfg)
		ui.Success("Application created and deploying")
	} else {
		// Trigger deploy for existing app
		fmt.Println()
		spinner = ui.NewSpinner("Triggering deployment...")
		spinner.Start()
		_, err := client.Deploy(appUUID, false)
		spinner.Stop()
		if err != nil {
			return fmt.Errorf("failed to trigger deploy: %w", err)
		}
		ui.Success("Deployment triggered")
	}

	// Watch deployment progress
	success := watchDeployment(client, appUUID)

	// Get app info for URL
	app, err := client.GetApplication(appUUID)
	if err == nil && app.FQDN != "" {
		fmt.Println()
		fmt.Printf("Deployed to: %s\n", app.FQDN)
	}

	if !success {
		return fmt.Errorf("deployment failed")
	}
	return nil
}

// watchDeployment polls the deployment status and displays build logs
// Returns true if deployment succeeded, false if it failed
func watchDeployment(client *api.Client, appUUID string) bool {
	fmt.Println()
	fmt.Println("Build logs:")
	fmt.Println(strings.Repeat("-", 50))

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
				fmt.Println()
				fmt.Println(strings.Repeat("-", 50))
				// Check application status to determine if deployment succeeded
				app, err := client.GetApplication(appUUID)
				if err != nil {
					if debug {
						fmt.Printf("[DEBUG] GetApplication error: %v\n", err)
					}
					ui.Warn("Deployment finished (could not verify status)")
					return true // Assume success if we can't check
				}
				appStatus := strings.ToLower(app.Status)
				if debug {
					fmt.Printf("[DEBUG] Application status: %s\n", appStatus)
				}
				// "running" means success, other statuses may indicate issues
				if appStatus == "running" {
					ui.Success("Deployment completed")
					return true
				} else if appStatus == "exited" || appStatus == "error" || appStatus == "failed" {
					ui.Error(fmt.Sprintf("Deployment failed (app status: %s)", app.Status))
					return false
				}
				// Other statuses (starting, etc.) - assume success
				ui.Success("Deployment completed")
				return true
			}
			// If we never saw a deployment and waited a while, give up
			if !hadDeployment && attempt >= 15 {
				fmt.Println()
				fmt.Println(strings.Repeat("-", 50))
				ui.Warn("No deployment found")
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
				fmt.Println()
				fmt.Println(strings.Repeat("-", 50))
				// Deployment finished - check app status to confirm success
				app, err := client.GetApplication(appUUID)
				if err == nil && strings.ToLower(app.Status) == "running" {
					ui.Success("Deployment completed")
					return true
				}
				ui.Success("Deployment completed")
				return true
			}
			if status == "failed" || status == "error" || status == "cancelled" {
				fmt.Println()
				fmt.Println(strings.Repeat("-", 50))
				ui.Error(fmt.Sprintf("Deployment %s", status))
				return false
			}
			// "running", "in_progress", "queued" etc. mean still deploying - keep waiting
		}

		// Fallback: check status from deployment list
		status := strings.ToLower(latest.Status)
		if status == "finished" {
			fmt.Println()
			fmt.Println(strings.Repeat("-", 50))
			ui.Success("Deployment completed")
			return true
		}
		if status == "failed" || status == "error" || status == "cancelled" {
			fmt.Println()
			fmt.Println(strings.Repeat("-", 50))
			ui.Error(fmt.Sprintf("Deployment %s", status))
			return false
		}

		time.Sleep(2 * time.Second)
		attempt++
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 50))
	ui.Warn("Deployment still in progress (timed out waiting)")
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

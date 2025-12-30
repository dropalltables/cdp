package deploy

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/detect"
	"github.com/dropalltables/cdp/internal/git"
	"github.com/dropalltables/cdp/internal/ui"
)

// DeployGit handles Git-based deployments
func DeployGit(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, prNumber int, verbose bool) error {
	ghClient := git.NewGitHubClient(globalCfg.GitHubToken)

	// Get GitHub user
	user, err := getGitHubUser(ghClient, verbose)
	if err != nil {
		return err
	}

	// Handle GitHub repository setup (if needed)
	repoName := projectCfg.GitHubRepo
	if strings.Contains(repoName, "/") {
		parts := strings.Split(repoName, "/")
		repoName = parts[len(parts)-1]
	}
	needsRepoCreation := !ghClient.RepoExists(user.Login, repoName)
	if err := handleGitHubRepoSetup(ghClient, projectCfg, user.Login, needsRepoCreation); err != nil {
		return err
	}

	// Handle GitHub App selection (if needed)
	if err := handleGitHubAppSelection(client, projectCfg, needsRepoCreation, verbose); err != nil {
		return err
	}

	// Execute deployment tasks
	tasks := buildGitDeploymentTasks(client, ghClient, globalCfg, projectCfg, user.Login, needsRepoCreation, verbose)

	if err := ui.RunTasksVerbose(tasks, verbose); err != nil {
		ui.Error("Deployment setup failed")
		return err
	}

	// Watch deployment
	ui.Info("Watching deployment...")

	success := WatchDeployment(client, projectCfg.AppUUID)

	if !success {
		ui.Error("Deployment failed")
		ui.Spacer()
		ui.NextSteps([]string{
			"Run 'cdp logs' to view deployment logs",
			"Check the Coolify dashboard for more details",
		})
		return fmt.Errorf("deployment failed")
	}

	// Get app info for URL
	ui.Success("Deployment complete")

	app, err := client.GetApplication(projectCfg.AppUUID)
	if err == nil && app.FQDN != "" {
		fmt.Println(ui.DimStyle.Render("  URL: " + app.FQDN))
	}

	return nil
}

func getGitHubUser(ghClient *git.GitHubClient, verbose bool) (*git.User, error) {
	var user *git.User
	err := ui.RunTasksVerbose([]ui.Task{
		{
			Name:         "github-check",
			ActiveName:   "Checking GitHub connection...",
			CompleteName: "Connected to GitHub",
			Action: func() error {
				var err error
				user, err = ghClient.GetUser()
				return err
			},
		},
	}, verbose)
	if err != nil {
		ui.Error("Failed to connect to GitHub")
		return nil, fmt.Errorf("failed to connect to GitHub: %w", err)
	}
	return user, nil
}

func handleGitHubRepoSetup(ghClient *git.GitHubClient, projectCfg *config.ProjectConfig, username string, needsRepoCreation bool) error {
	if !needsRepoCreation {
		return nil
	}

	// Ask for repo name
	repoName, err := ui.InputWithDefault("Repository name", projectCfg.GitHubRepo)
	if err != nil {
		return err
	}
	projectCfg.GitHubRepo = repoName

	// Ask for visibility
	visibilityOptions := []string{"Private", "Public"}
	visibility, err := ui.Select("Repository visibility", visibilityOptions)
	if err != nil {
		return err
	}
	projectCfg.GitHubPrivate = (visibility == "Private")

	return nil
}

func handleGitHubAppSelection(client *api.Client, projectCfg *config.ProjectConfig, needsRepoCreation bool, verbose bool) error {
	// Use saved GitHub App if available
	if projectCfg.GitHubAppUUID != "" {
		return nil
	}

	// Load GitHub Apps
	var githubApps []api.GitHubApp
	err := ui.RunTasksVerbose([]ui.Task{
		{
			Name:         "load-apps",
			ActiveName:   "Loading GitHub Apps...",
			CompleteName: "Loaded GitHub Apps",
			Action: func() error {
				var err error
				githubApps, err = client.ListGitHubApps()
				return err
			},
		},
	}, verbose)
	if err != nil {
		ui.Error("Failed to load GitHub Apps")
		ui.Dim("Configure a GitHub App in Coolify: Sources -> GitHub App")
		return fmt.Errorf("failed to list GitHub Apps: %w", err)
	}

	if len(githubApps) == 0 {
		ui.Error("No GitHub Apps configured in Coolify")
		ui.Dim("Add a GitHub App in Coolify: Sources -> GitHub App")
		return fmt.Errorf("no GitHub Apps configured")
	}

	// Select GitHub App
	var githubAppUUID string
	if len(githubApps) == 1 {
		githubAppUUID = githubApps[0].UUID
		ui.LogChoice("GitHub App", githubApps[0].Name)
	} else {
		// Build ordered options list with non-public GitHub apps first (as default)
		var appOptions []struct{ Key, Display string }
		
		// Add non-public apps first
		for _, app := range githubApps {
			if !isPublicGitHub(app.Name) {
				displayName := app.Name
				if app.Organization != "" {
					displayName = fmt.Sprintf("%s (%s)", app.Name, app.Organization)
				}
				appOptions = append(appOptions, struct{ Key, Display string }{Key: app.UUID, Display: displayName})
			}
		}
		
		// Then add public apps
		for _, app := range githubApps {
			if isPublicGitHub(app.Name) {
				displayName := app.Name
				if app.Organization != "" {
					displayName = fmt.Sprintf("%s (%s)", app.Name, app.Organization)
				}
				appOptions = append(appOptions, struct{ Key, Display string }{Key: app.UUID, Display: displayName})
			}
		}
		
		githubAppUUID, err = ui.SelectWithKeysOrdered("Select GitHub App", appOptions)
		if err != nil {
			return err
		}
	}

	// Save the selected GitHub App UUID
	projectCfg.GitHubAppUUID = githubAppUUID
	err = config.SaveProject(projectCfg)
	if err != nil {
		ui.Warning("Failed to save GitHub App selection")
	}

	return nil
}

// isPublicGitHub checks if a GitHub app is the public GitHub (not self-hosted)
func isPublicGitHub(appName string) bool {
	return strings.Contains(strings.ToLower(appName), "public") || 
		   strings.Contains(strings.ToLower(appName), "github.com")
}

func buildGitDeploymentTasks(
	client *api.Client,
	ghClient *git.GitHubClient,
	globalCfg *config.GlobalConfig,
	projectCfg *config.ProjectConfig,
	username string,
	needsRepoCreation bool,
	verbose bool,
) []ui.Task {
	tasks := []ui.Task{}

	// Create project and environment if needed
	needsProjectCreation := projectCfg.ProjectUUID == ""
	if needsProjectCreation {
		tasks = append(tasks, createProjectTask(client, projectCfg))
		tasks = append(tasks, setupEnvironmentTask(client, projectCfg))
	} else {
		tasks = append(tasks, checkEnvironmentTask(client, projectCfg))
	}

	// Create GitHub repo if needed
	if needsRepoCreation {
		tasks = append(tasks, createGitHubRepoTask(ghClient, projectCfg))
	}

	// Initialize git if needed
	if !git.IsRepo(".") {
		tasks = append(tasks, initGitTask())
	}

	// Create Coolify app if needed (before push so webhook works)
	if projectCfg.AppUUID == "" {
		tasks = append(tasks, createGitAppTask(client, projectCfg, username))
	}

	// Push code to GitHub and trigger deployment
	// Webhook triggers on push, but if no changes we trigger manually
	tasks = append(tasks, pushAndDeployTask(client, ghClient, globalCfg, projectCfg, username, verbose))

	return tasks
}

func createGitHubRepoTask(ghClient *git.GitHubClient, projectCfg *config.ProjectConfig) ui.Task {
	return ui.Task{
		Name:         "create-repo",
		ActiveName:   "Creating GitHub repository...",
		CompleteName: "Created GitHub repository",
		Action: func() error {
			// Create README if it doesn't exist
			_ = CreateReadmeIfMissing(projectCfg)

			// Extract just the repo name (not the owner/name format)
			repoName := projectCfg.GitHubRepo
			if strings.Contains(repoName, "/") {
				parts := strings.Split(repoName, "/")
				repoName = parts[len(parts)-1]
			}

			_, err := ghClient.CreateRepo(
				repoName,
				fmt.Sprintf("Deployment repository for %s", projectCfg.Name),
				projectCfg.GitHubPrivate,
			)
			if err != nil {
				return fmt.Errorf("failed to create GitHub repository %q: %w", projectCfg.GitHubRepo, err)
			}

			return config.SaveProject(projectCfg)
		},
	}
}

func initGitTask() ui.Task {
	return ui.Task{
		Name:         "init-git",
		ActiveName:   "Initializing git repository...",
		CompleteName: "Initialized git repository",
		Action: func() error {
			if err := git.Init("."); err != nil {
				return fmt.Errorf("failed to initialize git repository: %w", err)
			}
			return nil
		},
	}
}

func pushAndDeployTask(client *api.Client, ghClient *git.GitHubClient, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, username string, verbose bool) ui.Task {
	return ui.Task{
		Name:         "push-deploy",
		ActiveName:   "Pushing code to GitHub...",
		CompleteName: "Pushed code to GitHub",
		Action: func() error {
			// Extract just the repo name (projectCfg.GitHubRepo may contain owner/name or just name)
			repoName := projectCfg.GitHubRepo
			if strings.Contains(repoName, "/") {
				parts := strings.Split(repoName, "/")
				repoName = parts[len(parts)-1]
			}
			fullRepoName := fmt.Sprintf("%s/%s", username, repoName)

			// Use HTTPS URL without embedded token (more secure)
			remoteURL := fmt.Sprintf("https://github.com/%s.git", fullRepoName)
			if err := git.SetRemote(".", "origin", remoteURL); err != nil {
				return fmt.Errorf("failed to configure git remote: %w", err)
			}

			// Auto-commit any changes
			hadChanges := git.HasChanges(".")
			_ = git.AutoCommitVerbose(".", verbose)

			// Determine branch
			branch := projectCfg.Branch
			if branch == "" {
				b, _ := git.GetCurrentBranch(".")
				if b == "" {
					branch = config.DefaultBranch
				} else {
					branch = b
				}
			}

			// Push to GitHub - webhook triggers deployment if there are changes
			err := git.PushWithTokenVerbose(".", "origin", branch, globalCfg.GitHubToken, verbose)
			if err != nil {
				return err
			}

			// If no changes were committed, webhook won't fire - trigger manually
			if !hadChanges {
				_, err = client.Deploy(projectCfg.AppUUID, false, 0)
				if err != nil {
					return fmt.Errorf("failed to trigger deployment: %w", err)
				}
			}

			return nil
		},
	}
}

func createGitAppTask(client *api.Client, projectCfg *config.ProjectConfig, username string) ui.Task {
	return ui.Task{
		Name:         "create-app",
		ActiveName:   "Creating Coolify application...",
		CompleteName: "Created Coolify application",
		Action: func() error {
			buildPack := projectCfg.BuildPack
			if buildPack == "" {
				buildPack = detect.BuildPackNixpacks
			}

			port := projectCfg.Port
			if port == "" {
				port = config.DefaultPort
			}

			branch := projectCfg.Branch
			if branch == "" {
				b, _ := git.GetCurrentBranch(".")
				if b == "" {
					branch = config.DefaultBranch
				} else {
					branch = b
				}
			}

			// Extract just the repo name (projectCfg.GitHubRepo may contain owner/name or just name)
			repoName := projectCfg.GitHubRepo
			if strings.Contains(repoName, "/") {
				parts := strings.Split(repoName, "/")
				repoName = parts[len(parts)-1]
			}
			fullRepoName := fmt.Sprintf("%s/%s", username, repoName)

			// Use Coolify's static site feature for static builds
			isStatic := buildPack == detect.BuildPackStatic

			// Enable health check for static sites
			healthCheckEnabled := isStatic
			healthCheckPath := "/"

			resp, err := client.CreatePrivateGitHubApp(&api.CreatePrivateGitHubAppRequest{
				ProjectUUID:        projectCfg.ProjectUUID,
				ServerUUID:         projectCfg.ServerUUID,
				EnvironmentUUID:    projectCfg.EnvironmentUUID,
				GitHubAppUUID:      projectCfg.GitHubAppUUID,
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
			projectCfg.AppUUID = resp.UUID

			return config.SaveProject(projectCfg)
		},
	}
}


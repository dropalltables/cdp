package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/detect"
	"github.com/dropalltables/cdp/internal/docker"
	"github.com/dropalltables/cdp/internal/git"
	"github.com/dropalltables/cdp/internal/ui"
)

// FirstTimeSetup walks the user through initial project configuration.
func FirstTimeSetup(client *api.Client, globalCfg *config.GlobalConfig) (*config.ProjectConfig, error) {
	ui.Spacer()

	// Detect framework
	framework, err := detectFramework()
	if err != nil {
		return nil, err
	}

	// Choose deployment method
	deployMethod, err := chooseDeployMethod(globalCfg)
	if err != nil {
		return nil, err
	}

	// Select server
	serverUUID, err := selectServer(client)
	if err != nil {
		return nil, err
	}

	// Select or create project
	projectName, projectUUID, environmentUUID, err := selectOrCreateProject(client)
	if err != nil {
		return nil, err
	}

	// Advanced options
	advancedCfg, err := configureAdvancedOptions(deployMethod, framework)
	if err != nil {
		return nil, err
	}

	// Build project config
	projectCfg := buildProjectConfig(
		projectName,
		projectUUID,
		environmentUUID,
		serverUUID,
		deployMethod,
		framework,
		advancedCfg,
		globalCfg,
	)

	// Save project config
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "save-config",
			ActiveName:   "Saving configuration...",
			CompleteName: "Saved configuration",
			Action: func() error {
				return config.SaveProject(projectCfg)
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	ui.Spacer()
	ui.Success("Project configured successfully")

	return projectCfg, nil
}

func detectFramework() (*detect.FrameworkInfo, error) {
	var framework *detect.FrameworkInfo

	err := ui.RunTasks([]ui.Task{
		{
			Name:         "detect-framework",
			ActiveName:   "Analyzing project...",
			CompleteName: "Analyzed project",
			Action: func() error {
				var err error
				framework, err = detect.Detect(".")
				return err
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to detect framework: %w", err)
	}

	ui.LogChoice("Framework", framework.Name)

	// Display build settings inline
	if framework.InstallCommand != "" {
		ui.KeyValue("Install", framework.InstallCommand)
	}
	if framework.BuildCommand != "" {
		ui.KeyValue("Build", framework.BuildCommand)
	}
	if framework.StartCommand != "" {
		ui.KeyValue("Start", framework.StartCommand)
	}
	if framework.PublishDirectory != "" {
		ui.KeyValue("Output", framework.PublishDirectory)
	}

	editSettings, err := ui.Confirm("Customize build settings?")
	if err != nil {
		return nil, err
	}

	if editSettings {
		framework, err = editBuildSettings(framework)
		if err != nil {
			return nil, err
		}

		// Show updated configuration
		ui.Spacer()
		ui.Dim("Updated Configuration:")
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
	}

	return framework, nil
}

func editBuildSettings(f *detect.FrameworkInfo) (*detect.FrameworkInfo, error) {
	installCmd, err := ui.InputWithDefault("Install command", f.InstallCommand)
	if err != nil {
		return nil, err
	}
	f.InstallCommand = installCmd

	buildCmd, err := ui.InputWithDefault("Build command", f.BuildCommand)
	if err != nil {
		return nil, err
	}
	f.BuildCommand = buildCmd

	startCmd, err := ui.InputWithDefault("Start command", f.StartCommand)
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
			"Run 'cdp login' to configure authentication",
		})
		return "", fmt.Errorf("no deployment method configured")
	}

	if len(options) == 1 {
		ui.LogChoice("Deployment method", options[0])
		return optionMap[options[0]], nil
	}

	selected, err := ui.Select("Deployment method", options)
	if err != nil {
		return "", err
	}
	return optionMap[selected], nil
}

func selectServer(client *api.Client) (string, error) {
	var servers []api.Server
	err := ui.RunTasks([]ui.Task{
		{
			Name:         "load-servers",
			ActiveName:   "Loading servers...",
			CompleteName: "Loaded servers",
			Action: func() error {
				var err error
				servers, err = client.ListServers()
				return err
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to list servers: %w", err)
	}

	if len(servers) == 0 {
		ui.Error("No servers found in Coolify")
		ui.Dim("Add a server in your Coolify dashboard first")
		return "", fmt.Errorf("no servers available")
	}

	serverOptions := make(map[string]string)
	for _, s := range servers {
		displayName := s.Name
		if s.IP != "" {
			displayName = fmt.Sprintf("%s (%s)", s.Name, s.IP)
		}
		serverOptions[s.UUID] = displayName
	}

	serverUUID, err := ui.SelectWithKeys("Server", serverOptions)
	if err != nil {
		return "", err
	}

	return serverUUID, nil
}

func selectOrCreateProject(client *api.Client) (projectName, projectUUID, environmentUUID string, err error) {
	var projects []api.Project
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "load-projects",
			ActiveName:   "Loading projects...",
			CompleteName: "Loaded projects",
			Action: func() error {
				var err error
				projects, err = client.ListProjects()
				return err
			},
		},
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to list projects: %w", err)
	}

	projectOptions := make([]string, 0, len(projects)+1)
	projectOptions = append(projectOptions, "+ Create new project")
	projectMap := make(map[string]api.Project)
	for _, p := range projects {
		projectOptions = append(projectOptions, p.Name)
		projectMap[p.Name] = p
	}

	selectedProject, err := ui.Select("Project", projectOptions)
	if err != nil {
		return "", "", "", err
	}

	if selectedProject == "+ Create new project" {
		workingDirName := getWorkingDirName()
		projectName, err = ui.InputWithDefault("Project name", workingDirName)
		if err != nil {
			return "", "", "", err
		}
		projectUUID = ""
		environmentUUID = ""
	} else {
		project := projectMap[selectedProject]
		projectName = selectedProject
		projectUUID = project.UUID
		environmentUUID = ""
	}

	return projectName, projectUUID, environmentUUID, nil
}

type advancedConfig struct {
	Port     string
	Platform string
	Branch   string
	Domain   string
}

func configureAdvancedOptions(deployMethod string, framework *detect.FrameworkInfo) (*advancedConfig, error) {
	configureAdvanced, err := ui.Confirm("Configure advanced options")
	if err != nil {
		return nil, err
	}

	cfg := &advancedConfig{
		Port:     framework.Port,
		Platform: config.DefaultPlatform,
		Branch:   config.DefaultBranch,
		Domain:   "",
	}

	if cfg.Port == "" {
		cfg.Port = config.DefaultPort
	}

	if !configureAdvanced {
		return cfg, nil
	}

	cfg.Port, err = ui.InputWithDefault("Application port", cfg.Port)
	if err != nil {
		return nil, err
	}

	if deployMethod == config.DeployMethodDocker {
		platformOptions := []string{"linux/amd64 (Intel/AMD)", "linux/arm64 (ARM)"}
		platformChoice, err := ui.Select("Target platform", platformOptions)
		if err != nil {
			return nil, err
		}
		if strings.Contains(platformChoice, "arm64") {
			cfg.Platform = "linux/arm64"
		}
	}

	if deployMethod == config.DeployMethodGit {
		cfg.Branch, err = ui.InputWithDefault("Git branch", cfg.Branch)
		if err != nil {
			return nil, err
		}
	}

	useDomain, err := ui.Confirm("Configure custom domain")
	if err != nil {
		return nil, err
	}
	if useDomain {
		cfg.Domain, err = ui.Input("Domain", "app.example.com")
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func buildProjectConfig(
	projectName, projectUUID, environmentUUID, serverUUID, deployMethod string,
	framework *detect.FrameworkInfo,
	advancedCfg *advancedConfig,
	globalCfg *config.GlobalConfig,
) *config.ProjectConfig {
	projectCfg := &config.ProjectConfig{
		Name:            projectName,
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
		Port:            advancedCfg.Port,
		Platform:        advancedCfg.Platform,
		Branch:          advancedCfg.Branch,
		Domain:          advancedCfg.Domain,
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

	return projectCfg
}

func getWorkingDirName() string {
	dir, err := os.Getwd()
	if err != nil {
		return "app"
	}
	return filepath.Base(dir)
}

// CreateReadmeIfMissing creates a README.md file if one doesn't exist
func CreateReadmeIfMissing(cfg *config.ProjectConfig) error {
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

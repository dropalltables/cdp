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
	result, err := detectFramework()
	if err != nil {
		return nil, err
	}

	deployMethod, err := chooseDeployMethod(globalCfg)
	if err != nil {
		return nil, err
	}

	serverUUID, err := selectServer(client)
	if err != nil {
		return nil, err
	}

	projectName, projectUUID, environmentUUID, err := selectOrCreateProject(client)
	if err != nil {
		return nil, err
	}

	advancedCfg, err := configureAdvancedOptions(deployMethod, result)
	if err != nil {
		return nil, err
	}

	projectCfg := buildProjectConfig(
		projectName,
		projectUUID,
		environmentUUID,
		serverUUID,
		deployMethod,
		result,
		advancedCfg,
		globalCfg,
	)

	if err := config.SaveProject(projectCfg); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	ui.Success("Project configured successfully")
	return projectCfg, nil
}

func detectFramework() (*detect.Result, error) {
	var result *detect.Result

	err := ui.RunTasks([]ui.Task{
		{
			Name:         "detect-framework",
			ActiveName:   "Analyzing project...",
			CompleteName: "Analyzed project",
			Action: func() error {
				var err error
				result, err = detect.Detect(".")
				return err
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to detect framework: %w", err)
	}

	ui.LogChoice("Framework", result.Framework)

	// Show coolpack detection info for Node.js projects
	if result.Kind == "node" && result.PackageManager != "" {
		pmInfo := result.PackageManager
		if result.PackageManagerVersion != "" {
			pmInfo += " " + result.PackageManagerVersion
		}
		ui.KeyValue("Package manager", pmInfo)

		if result.LanguageVersion != "" {
			ui.KeyValue("Node version", result.LanguageVersion)
		}
		if result.IsStatic {
			ui.KeyValue("Output type", "Static")
		} else {
			ui.KeyValue("Output type", "Server")
		}
	}

	if result.InstallCommand != "" {
		ui.KeyValue("Install", result.InstallCommand)
	}
	if result.BuildCommand != "" {
		ui.KeyValue("Build", result.BuildCommand)
	}
	if result.StartCommand != "" {
		ui.KeyValue("Start", result.StartCommand)
	}
	if result.PublishDirectory != "" {
		ui.KeyValue("Output", result.PublishDirectory)
	}

	ui.Spacer()

	editSettings, err := ui.Confirm("Customize build settings?")
	if err != nil {
		return nil, err
	}

	if editSettings {
		result, err = editBuildSettings(result)
		if err != nil {
			return nil, err
		}

		ui.Spacer()
		if result.InstallCommand != "" {
			ui.KeyValue("Install", ui.CodeStyle.Render(result.InstallCommand))
		}
		if result.BuildCommand != "" {
			ui.KeyValue("Build", ui.CodeStyle.Render(result.BuildCommand))
		}
		if result.StartCommand != "" {
			ui.KeyValue("Start", ui.CodeStyle.Render(result.StartCommand))
		}
		if result.PublishDirectory != "" {
			ui.KeyValue("Publish dir", result.PublishDirectory)
		}
	}

	return result, nil
}

func editBuildSettings(r *detect.Result) (*detect.Result, error) {
	var err error

	r.InstallCommand, err = ui.InputWithDefault("Install command", r.InstallCommand)
	if err != nil {
		return nil, err
	}

	r.BuildCommand, err = ui.InputWithDefault("Build command", r.BuildCommand)
	if err != nil {
		return nil, err
	}

	r.StartCommand, err = ui.InputWithDefault("Start command", r.StartCommand)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func chooseDeployMethod(globalCfg *config.GlobalConfig) (string, error) {
	var options []string
	optionMap := map[string]string{}

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

	return ui.SelectWithKeys("Server", serverOptions)
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

	projectOptions := []string{"+ Create new project"}
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
		projectName, err = ui.InputWithDefault("Project name", getWorkingDirName())
		if err != nil {
			return "", "", "", err
		}
	} else {
		project := projectMap[selectedProject]
		projectName = selectedProject
		projectUUID = project.UUID
	}

	return projectName, projectUUID, environmentUUID, nil
}

type advancedConfig struct {
	Port     string
	Platform string
	Branch   string
	Domain   string
}

func configureAdvancedOptions(deployMethod string, result *detect.Result) (*advancedConfig, error) {
	configureAdvanced, err := ui.Confirm("Configure advanced options")
	if err != nil {
		return nil, err
	}

	cfg := &advancedConfig{
		Port:     result.Port,
		Platform: config.DefaultPlatform,
		Branch:   config.DefaultBranch,
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
	result *detect.Result,
	advancedCfg *advancedConfig,
	globalCfg *config.GlobalConfig,
) *config.ProjectConfig {
	projectCfg := &config.ProjectConfig{
		Name:            projectName,
		DeployMethod:    deployMethod,
		ProjectUUID:     projectUUID,
		ServerUUID:      serverUUID,
		EnvironmentUUID: environmentUUID,
		Framework:       result.Framework,
		InstallCommand:  result.InstallCommand,
		BuildCommand:    result.BuildCommand,
		StartCommand:    result.StartCommand,
		PublishDir:      result.PublishDirectory,
		Port:            advancedCfg.Port,
		Platform:        advancedCfg.Platform,
		Branch:          advancedCfg.Branch,
		Domain:          advancedCfg.Domain,
	}

	// Store Node.js specific info
	if result.Kind == "node" {
		projectCfg.PackageManager = result.PackageManager
		projectCfg.PackageManagerVersion = result.PackageManagerVersion
		projectCfg.NodeVersion = result.LanguageVersion
		projectCfg.CoolpackPlan = result.MarshalPlan()
	}

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
		return nil
	}

	content := fmt.Sprintf("# %s\n\n%s application deployed to Coolify.\n", cfg.Name, cfg.Framework)
	return os.WriteFile(readmePath, []byte(content), 0644)
}

// CreateGitignoreIfMissing creates a .gitignore file if one doesn't exist
func CreateGitignoreIfMissing(cfg *config.ProjectConfig) error {
	gitignorePath := filepath.Join(".", ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil
	}

	var content string
	switch cfg.Framework {
	case "Node.js", "Express", "Next.js", "Nuxt", "Remix", "Astro", "SvelteKit", "Vite", "React", "Vue", "Angular":
		content = `node_modules/
dist/
build/
.next/
.nuxt/
.output/
.env
.env.local
*.log
`
	case "Go":
		content = `bin/
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
.env
`
	case "Python", "Django", "Flask", "FastAPI":
		content = `__pycache__/
*.py[cod]
*$py.class
*.so
.Python
venv/
.env
*.egg-info/
dist/
build/
`
	case "Hugo":
		content = `public/
resources/
.hugo_build.lock
`
	default:
		content = `.env
.env.local
*.log
`
	}

	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

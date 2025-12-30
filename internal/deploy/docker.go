package deploy

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/detect"
	"github.com/dropalltables/cdp/internal/docker"
	"github.com/dropalltables/cdp/internal/ui"
)

// DeployDocker handles Docker-based deployments
func DeployDocker(client *api.Client, globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, prNumber int, verbose bool) error {
	// Generate tag based on PR number (0 = production, >0 = preview)
	deployType := "production"
	if prNumber > 0 {
		deployType = fmt.Sprintf("pr-%d", prNumber)
	}
	tag := docker.GenerateTag(deployType)

	needsProjectCreation := projectCfg.ProjectUUID == ""

	ui.KeyValue("Image", projectCfg.DockerImage)
	ui.KeyValue("Tag", tag)
	ui.KeyValue("Platform", projectCfg.Platform)

	// Build Docker image
	if err := buildDockerImage(projectCfg, tag, verbose); err != nil {
		return err
	}

	ui.Info("Deploying to Coolify")

	tasks := buildDockerDeploymentTasks(client, globalCfg, projectCfg, tag, needsProjectCreation, verbose)

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

func buildDockerImage(projectCfg *config.ProjectConfig, tag string, verbose bool) error {
	framework := &detect.FrameworkInfo{
		Name:             projectCfg.Framework,
		InstallCommand:   projectCfg.InstallCommand,
		BuildCommand:     projectCfg.BuildCommand,
		StartCommand:     projectCfg.StartCommand,
		PublishDirectory: projectCfg.PublishDir,
	}

	// Use spinner for build unless verbose mode is enabled
	var err error
	if !verbose {
		buildTask := ui.Task{
			Name:         "build-image",
			ActiveName:   "Building Docker image...",
			CompleteName: "Image built successfully",
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
		fmt.Println("* Building Docker image...")
		err = docker.Build(&docker.BuildOptions{
			Dir:       ".",
			ImageName: projectCfg.DockerImage,
			Tag:       tag,
			Framework: framework,
			Platform:  projectCfg.Platform,
			Verbose:   true,
		})
	}

	if err != nil {
		ui.Error("Build failed")
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}

func buildDockerDeploymentTasks(
	client *api.Client,
	globalCfg *config.GlobalConfig,
	projectCfg *config.ProjectConfig,
	tag string,
	needsProjectCreation bool,
	verbose bool,
) []ui.Task {
	tasks := []ui.Task{}

	// Create project and environment if needed
	if needsProjectCreation {
		tasks = append(tasks, createProjectTask(client, projectCfg))
		tasks = append(tasks, setupEnvironmentTask(client, projectCfg))
	} else {
		tasks = append(tasks, checkEnvironmentTask(client, projectCfg))
	}

	// Push image
	tasks = append(tasks, pushImageTask(globalCfg, projectCfg, tag, verbose))

	// Create app if needed
	if projectCfg.AppUUID == "" {
		tasks = append(tasks, createDockerAppTask(client, projectCfg, tag))
	}

	// Trigger deployment
	tasks = append(tasks, triggerDeploymentTask(client, projectCfg, tag))

	return tasks
}

func createProjectTask(client *api.Client, projectCfg *config.ProjectConfig) ui.Task {
	return ui.Task{
		Name:         "create-project",
		ActiveName:   "Creating Coolify project...",
		CompleteName: "Created Coolify project",
		Action: func() error {
			newProject, err := client.CreateProject(projectCfg.Name, "Created by CDP")
			if err != nil {
				return fmt.Errorf("failed to create Coolify project %q: %w", projectCfg.Name, err)
			}
			projectCfg.ProjectUUID = newProject.UUID
			return nil
		},
	}
}

func setupEnvironmentTask(client *api.Client, projectCfg *config.ProjectConfig) ui.Task {
	return ui.Task{
		Name:         "setup-env",
		ActiveName:   "Setting up environment...",
		CompleteName: "Set up environment",
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
	}
}

func checkEnvironmentTask(client *api.Client, projectCfg *config.ProjectConfig) ui.Task {
	return ui.Task{
		Name:         "check-env",
		ActiveName:   "Checking environment...",
		CompleteName: "Environment ready",
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
	}
}

func pushImageTask(globalCfg *config.GlobalConfig, projectCfg *config.ProjectConfig, tag string, verbose bool) ui.Task {
	return ui.Task{
		Name:         "push-image",
		ActiveName:   "Pushing image to registry...",
		CompleteName: "Pushed image to registry",
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
	}
}

func createDockerAppTask(client *api.Client, projectCfg *config.ProjectConfig, tag string) ui.Task {
	return ui.Task{
		Name:         "create-app",
		ActiveName:   "Creating Coolify application...",
		CompleteName: "Created Coolify application",
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
			projectCfg.AppUUID = resp.UUID

			return config.SaveProject(projectCfg)
		},
	}
}

func triggerDeploymentTask(client *api.Client, projectCfg *config.ProjectConfig, tag string) ui.Task {
	return ui.Task{
		Name:         "trigger-deploy",
		ActiveName:   "Triggering deployment...",
		CompleteName: "Triggered deployment",
		Action: func() error {
			if err := client.UpdateApplication(projectCfg.AppUUID, map[string]interface{}{
				"docker_registry_image_tag": tag,
			}); err != nil {
				return fmt.Errorf("failed to update application image tag: %w", err)
			}

			_, err := client.Deploy(projectCfg.AppUUID, false, 0)
			if err != nil {
				return fmt.Errorf("failed to trigger deployment: %w", err)
			}
			return nil
		},
	}
}

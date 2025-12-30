package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/docker"
	"github.com/dropalltables/cdp/internal/git"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:     "health",
	Aliases: []string{"whoami"},
	Short:   "Check service connectivity",
	Long:    "Verify connections to Coolify, GitHub, and Docker registry.",
	RunE:    runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	type checkResult struct {
		name   string
		status string
		detail string
		ok     bool
	}

	results := []checkResult{}
	var coolifyClient *api.Client
	_ = coolifyClient // Will be populated but may not be used

	tasks := []ui.Task{}

	// Coolify check task
	tasks = append(tasks, ui.Task{
		Name:         "check-coolify",
		ActiveName:   "Checking Coolify...",
		CompleteName: "Coolify connected",
		Action: func() error {
			if cfg.CoolifyURL == "" || cfg.CoolifyToken == "" {
				results = append(results, checkResult{
					name:   "Coolify",
					status: "Not configured",
					detail: cfg.CoolifyURL,
					ok:     false,
				})
				return nil
			}
			coolifyClient = api.NewClient(cfg.CoolifyURL, cfg.CoolifyToken)
			if err := coolifyClient.HealthCheck(); err != nil {
				results = append(results, checkResult{
					name:   "Coolify",
					status: "Connection failed",
					detail: cfg.CoolifyURL,
					ok:     false,
				})
				return nil
			}
			results = append(results, checkResult{
				name:   "Coolify",
				status: "Connected",
				detail: cfg.CoolifyURL,
				ok:     true,
			})
			return nil
		},
	})

	// GitHub check task
	tasks = append(tasks, ui.Task{
		Name:         "check-github",
		ActiveName:   "Checking GitHub...",
		CompleteName: "GitHub authenticated",
		Action: func() error {
			if cfg.GitHubToken == "" {
				results = append(results, checkResult{
					name:   "GitHub",
					status: "Not configured",
					detail: "-",
					ok:     false,
				})
				return nil
			}
			ghClient := git.NewGitHubClient(cfg.GitHubToken)
			user, err := ghClient.GetUser()
			if err != nil {
				results = append(results, checkResult{
					name:   "GitHub",
					status: "Authentication failed",
					detail: "-",
					ok:     false,
				})
				return nil
			}
			results = append(results, checkResult{
				name:   "GitHub",
				status: "Authenticated",
				detail: user.Login,
				ok:     true,
			})
			return nil
		},
	})

	// Docker check task
	tasks = append(tasks, ui.Task{
		Name:         "check-docker",
		ActiveName:   "Checking Docker...",
		CompleteName: "Docker running",
		Action: func() error {
			if !docker.IsDockerAvailable() {
				results = append(results, checkResult{
					name:   "Docker",
					status: "Not running",
					detail: "local",
					ok:     false,
				})
			} else {
				results = append(results, checkResult{
					name:   "Docker",
					status: "Running",
					detail: "local",
					ok:     true,
				})
			}
			return nil
		},
	})

	// Docker Registry check task
	tasks = append(tasks, ui.Task{
		Name:         "check-registry",
		ActiveName:   "Checking Docker Registry...",
		CompleteName: "Docker Registry authenticated",
		Action: func() error {
			if cfg.DockerRegistry == nil {
				results = append(results, checkResult{
					name:   "Docker Registry",
					status: "Not configured",
					detail: "-",
					ok:     false,
				})
				return nil
			}
			if !docker.IsDockerAvailable() {
				results = append(results, checkResult{
					name:   "Docker Registry",
					status: "Skipped",
					detail: "Docker not running",
					ok:     false,
				})
				return nil
			}
			err := docker.VerifyLogin(
				cfg.DockerRegistry.URL,
				cfg.DockerRegistry.Username,
				cfg.DockerRegistry.Password,
			)
			if err != nil {
				results = append(results, checkResult{
					name:   "Docker Registry",
					status: "Authentication failed",
					detail: cfg.DockerRegistry.URL,
					ok:     false,
				})
				return nil
			}
			results = append(results, checkResult{
				name:   "Docker Registry",
				status: "Authenticated",
				detail: cfg.DockerRegistry.URL,
				ok:     true,
			})
			return nil
		},
	})

	// Run all checks
	if err := ui.RunTasks(tasks); err != nil {
		ui.Error("Health check failed")
		return err
	}

	// Check if all healthy
	allHealthy := true
	for _, r := range results {
		if !r.ok {
			allHealthy = false
			break
		}
	}

	if !allHealthy {
		ui.Spacer()
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s login' to configure authentication", execName()),
		})
	}

	return nil // Don't return error, just show status
}

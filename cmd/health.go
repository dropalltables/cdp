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
	Use:   "health",
	Short: "Check service connectivity",
	Long:  "Verify connections to Coolify, GitHub, and Docker registry.",
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ui.Section("Health Check")

	type check struct {
		name   string
		status string
		detail string
		ok     bool
	}

	checks := []check{}
	allHealthy := true

	// Check Coolify
	if cfg.CoolifyURL == "" || cfg.CoolifyToken == "" {
		checks = append(checks, check{
			name:   "Coolify",
			status: "Not configured",
			detail: cfg.CoolifyURL,
			ok:     false,
		})
		allHealthy = false
	} else {
		client := api.NewClient(cfg.CoolifyURL, cfg.CoolifyToken)
		if err := client.HealthCheck(); err != nil {
			checks = append(checks, check{
				name:   "Coolify",
				status: "Connection failed",
				detail: cfg.CoolifyURL,
				ok:     false,
			})
			allHealthy = false
		} else {
			checks = append(checks, check{
				name:   "Coolify",
				status: "Connected",
				detail: cfg.CoolifyURL,
				ok:     true,
			})
		}
	}

	// Check GitHub
	if cfg.GitHubToken == "" {
		checks = append(checks, check{
			name:   "GitHub",
			status: "Not configured",
			detail: "-",
			ok:     false,
		})
	} else {
		ghClient := git.NewGitHubClient(cfg.GitHubToken)
		user, err := ghClient.GetUser()
		if err != nil {
			checks = append(checks, check{
				name:   "GitHub",
				status: "Authentication failed",
				detail: "-",
				ok:     false,
			})
			allHealthy = false
		} else {
			checks = append(checks, check{
				name:   "GitHub",
				status: "Authenticated",
				detail: user.Login,
				ok:     true,
			})
		}
	}

	// Check Docker (local)
	if !docker.IsDockerAvailable() {
		checks = append(checks, check{
			name:   "Docker",
			status: "Not running",
			detail: "local",
			ok:     false,
		})
	} else {
		checks = append(checks, check{
			name:   "Docker",
			status: "Running",
			detail: "local",
			ok:     true,
		})
	}

	// Check Docker Registry
	if cfg.DockerRegistry == nil {
		checks = append(checks, check{
			name:   "Docker Registry",
			status: "Not configured",
			detail: "-",
			ok:     false,
		})
	} else {
		if !docker.IsDockerAvailable() {
			checks = append(checks, check{
				name:   "Docker Registry",
				status: "Skipped",
				detail: "Docker not running",
				ok:     false,
			})
		} else {
			err := docker.VerifyLogin(
				cfg.DockerRegistry.URL,
				cfg.DockerRegistry.Username,
				cfg.DockerRegistry.Password,
			)
			if err != nil {
				checks = append(checks, check{
					name:   "Docker Registry",
					status: "Authentication failed",
					detail: cfg.DockerRegistry.URL,
					ok:     false,
				})
				allHealthy = false
			} else {
				checks = append(checks, check{
					name:   "Docker Registry",
					status: "Authenticated",
					detail: cfg.DockerRegistry.URL,
					ok:     true,
				})
			}
		}
	}

	// Display results as table
	headers := []string{"Service", "Status", "Details"}
	rows := [][]string{}

	for _, c := range checks {
		statusDisplay := c.status
		if c.ok {
			statusDisplay = ui.SuccessStyle.Render(ui.IconSuccess + " " + c.status)
		} else {
			statusDisplay = ui.DimStyle.Render(ui.IconDot + " " + c.status)
		}
		
		rows = append(rows, []string{
			c.name,
			statusDisplay,
			c.detail,
		})
	}

	ui.Table(headers, rows)
	ui.Spacer()

	if allHealthy {
		ui.Success("All services are operational")
		return nil
	}

	ui.Warning("Some services need attention")
	ui.NextSteps([]string{
		fmt.Sprintf("Run '%s login' to configure authentication", execName()),
	})

	return nil // Don't return error, just show status
}

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
	Short: "Check connectivity to all services",
	Long:  "Verify connections to Coolify, GitHub, and Docker registry.",
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("Health Check")
	fmt.Println()

	allHealthy := true

	// Check Coolify
	fmt.Print("Coolify:         ")
	if cfg.CoolifyURL == "" || cfg.CoolifyToken == "" {
		ui.Dim("not configured")
	} else {
		client := api.NewClient(cfg.CoolifyURL, cfg.CoolifyToken)
		if err := client.HealthCheck(); err != nil {
			ui.Error("connection failed")
			allHealthy = false
		} else {
			ui.Success("connected")
		}
	}

	// Check GitHub
	fmt.Print("GitHub:          ")
	if cfg.GitHubToken == "" {
		ui.Dim("not configured")
	} else {
		ghClient := git.NewGitHubClient(cfg.GitHubToken)
		user, err := ghClient.GetUser()
		if err != nil {
			ui.Error("authentication failed")
			allHealthy = false
		} else {
			ui.Success(fmt.Sprintf("authenticated as %s", user.Login))
		}
	}

	// Check Docker
	fmt.Print("Docker (local):  ")
	if !docker.IsDockerAvailable() {
		ui.Warn("not running")
	} else {
		ui.Success("running")
	}

	// Check Docker Registry
	fmt.Print("Docker registry: ")
	if cfg.DockerRegistry == nil {
		ui.Dim("not configured")
	} else {
		if !docker.IsDockerAvailable() {
			ui.Dim("skipped (Docker not running)")
		} else {
			err := docker.VerifyLogin(
				cfg.DockerRegistry.URL,
				cfg.DockerRegistry.Username,
				cfg.DockerRegistry.Password,
			)
			if err != nil {
				ui.Error("authentication failed")
				allHealthy = false
			} else {
				ui.Success(fmt.Sprintf("authenticated (%s)", cfg.DockerRegistry.URL))
			}
		}
	}

	fmt.Println()
	if allHealthy {
		ui.Success("All configured services are healthy")
	} else {
		ui.Warn("Some services have issues")
	}

	return nil
}

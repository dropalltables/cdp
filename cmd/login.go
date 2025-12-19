package cmd

import (
	"fmt"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/docker"
	"github.com/dropalltables/cdp/internal/git"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Coolify",
	Long: `Authenticate with your Coolify instance and optionally set up 
GitHub and Docker registry integrations.

Required:
  • Coolify URL
  • Coolify API token (from Settings → API Tokens)

Optional:
  • GitHub personal access token (for git-based deployments)
  • Docker registry credentials (for container-based deployments)`,
	RunE: runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Load existing config if any
	cfg, err := config.LoadGlobal()
	if err != nil {
		cfg = &config.GlobalConfig{}
	}

	ui.Section("Coolify Authentication")

	// Step 1: Coolify credentials
	coolifyURL, err := ui.Input("Coolify URL", "https://coolify.example.com")
	if err != nil {
		return err
	}
	coolifyURL = strings.TrimSuffix(coolifyURL, "/")
	if coolifyURL == "" {
		return fmt.Errorf("Coolify URL is required")
	}

	ui.Spacer()
	ui.Dim("→ Get your API token from Settings → API Tokens in Coolify")
	token, err := ui.Password("API Token")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("API token is required")
	}

	// Validate credentials
	ui.Spacer()
	ui.Info("Connecting to Coolify...")
	client := api.NewClient(coolifyURL, token)
	if err := client.HealthCheck(); err != nil {
		ui.Error("Connection failed")
		return fmt.Errorf("failed to connect: %w", err)
	}
	ui.Success("Connected to Coolify")

	// Save base credentials
	cfg.CoolifyURL = coolifyURL
	cfg.CoolifyToken = token

	// Step 2: Optional GitHub setup
	ui.Section("GitHub Integration (Optional)")
	ui.Dim("Enable git-based deployments with automatic repository management")
	ui.Spacer()

	setupGitHub, err := ui.Confirm("Configure GitHub?")
	if err != nil {
		return err
	}

	if setupGitHub {
		ui.Spacer()
		ui.Dim("→ Create a token at https://github.com/settings/tokens")
		ui.Dim("  Required scope: repo")
		ui.Spacer()

		githubToken, err := ui.Password("GitHub Token")
		if err != nil {
			return err
		}
		if githubToken != "" {
			// Verify GitHub token
			ui.Info("Verifying GitHub token...")
			ghClient := git.NewGitHubClient(githubToken)
			user, err := ghClient.GetUser()
			if err != nil {
				ui.Warning("GitHub verification failed: " + err.Error())
			} else {
				ui.Success("GitHub token verified")
				cfg.GitHubToken = githubToken
				ui.Spacer()
				ui.KeyValue("GitHub user", user.Login)
			}
		}
	}

	// Step 3: Optional Docker registry setup
	ui.Section("Docker Registry (Optional)")
	ui.Dim("Enable container-based deployments with private registries")
	ui.Spacer()

	setupDocker, err := ui.Confirm("Configure Docker registry?")
	if err != nil {
		return err
	}

	if setupDocker {
		if !docker.IsDockerAvailable() {
			ui.Warning("Docker is not running")
			ui.Dim("Start Docker Desktop and run 'cdp login' again to configure registry")
		} else {
			ui.Spacer()
			registryURL, err := ui.InputWithDefault("Registry URL", "ghcr.io")
			if err != nil {
				return err
			}
			username, err := ui.Input("Username", "")
			if err != nil {
				return err
			}
			password, err := ui.Password("Password/Token")
			if err != nil {
				return err
			}

			if registryURL != "" && username != "" && password != "" {
				ui.Spacer()
				ui.Info("Verifying registry credentials...")
				err := docker.VerifyLogin(registryURL, username, password)
				if err != nil {
					ui.Warning("Registry verification failed: " + err.Error())
				} else {
					ui.Success("Registry credentials verified")
					cfg.DockerRegistry = &config.DockerRegistry{
						URL:      registryURL,
						Username: username,
						Password: password,
					}

					ui.Spacer()
					ui.Warning("Server Setup Required")
					ui.Spacer()
					ui.Print("To enable Coolify to pull from your registry, run this on your Coolify server:")
					ui.Spacer()
					ui.Code(fmt.Sprintf("echo '%s' | docker login %s -u %s --password-stdin", password, registryURL, username))
					ui.Spacer()
				}
			}
		}
	}

	// Save config
	if err := config.SaveGlobal(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Show summary
	ui.Divider()
	ui.Success("Authentication configured")
	ui.Spacer()
	ui.KeyValue("Coolify URL", coolifyURL)

	if cfg.GitHubToken != "" {
		ui.KeyValue("GitHub", "configured")
	}
	if cfg.DockerRegistry != nil {
		ui.KeyValue("Docker registry", cfg.DockerRegistry.URL)
	}

	ui.NextSteps([]string{
		fmt.Sprintf("Run '%s' in a project directory to deploy", execName()),
		fmt.Sprintf("Run '%s health' to verify all connections", execName()),
	})

	return nil
}

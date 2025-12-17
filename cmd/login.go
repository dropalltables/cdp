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
	Short: "Log in to Coolify",
	Long: `Authenticate with your Coolify instance.

You'll need:
- Your Coolify URL (e.g., https://coolify.example.com)
- An API token from Keys & Tokens in your Coolify dashboard

Optionally, you can also set up:
- GitHub token for git-based deployments
- Docker registry credentials for docker-based deployments`,
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

	fmt.Println("Log in to Coolify")
	fmt.Println()

	// Get Coolify URL
	coolifyURL, err := ui.Input("Coolify URL", "https://coolify.example.com")
	if err != nil {
		return err
	}
	coolifyURL = strings.TrimSuffix(coolifyURL, "/")
	if coolifyURL == "" {
		return fmt.Errorf("Coolify URL is required")
	}

	// Get API token
	fmt.Println()
	ui.Dim("Get your API token from Keys & Tokens in your Coolify dashboard")
	token, err := ui.Password("API Token")
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("API token is required")
	}

	// Validate credentials
	fmt.Println()
	spinner := ui.NewSpinner("Verifying credentials...")
	spinner.Start()

	client := api.NewClient(coolifyURL, token)
	if err := client.HealthCheck(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect to Coolify: %w", err)
	}
	spinner.Stop()
	ui.Success("Connected to Coolify")

	// Save base credentials
	cfg.CoolifyURL = coolifyURL
	cfg.CoolifyToken = token

	// Ask about GitHub token for git-based deployments
	fmt.Println()
	setupGitHub, err := ui.Confirm("Set up GitHub for git-based deployments?")
	if err != nil {
		return err
	}
	if setupGitHub {
		fmt.Println()
		ui.Dim("Create a token at https://github.com/settings/tokens with 'repo' scope")
		githubToken, err := ui.Password("GitHub Token")
		if err != nil {
			return err
		}
		if githubToken != "" {
			// Verify GitHub token
			spinner = ui.NewSpinner("Verifying GitHub token...")
			spinner.Start()
			ghClient := git.NewGitHubClient(githubToken)
			user, err := ghClient.GetUser()
			spinner.Stop()
			if err != nil {
				ui.Warn("GitHub token verification failed")
				ui.Dim(fmt.Sprintf("Error: %v", err))
			} else {
				cfg.GitHubToken = githubToken
				ui.Success(fmt.Sprintf("GitHub authenticated as %s", user.Login))
			}
		}
	}

	// Ask about Docker registry for docker-based deployments
	fmt.Println()
	setupDocker, err := ui.Confirm("Set up Docker registry for docker-based deployments?")
	if err != nil {
		return err
	}
	if setupDocker {
		if !docker.IsDockerAvailable() {
			ui.Warn("Docker is not running - start Docker Desktop first")
		} else {
			fmt.Println()
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
				// Verify Docker registry locally
				fmt.Println()
				spinner = ui.NewSpinner("Verifying Docker registry credentials...")
				spinner.Start()
				err := docker.VerifyLogin(registryURL, username, password)
				spinner.Stop()
				if err != nil {
					ui.Warn("Docker registry verification failed")
					ui.Dim(fmt.Sprintf("Error: %v", err))
				} else {
					ui.Success("Docker registry verified locally")

					cfg.DockerRegistry = &config.DockerRegistry{
						URL:      registryURL,
						Username: username,
						Password: password,
					}

					// Show Coolify server setup guide
					fmt.Println()
					fmt.Println(strings.Repeat("=", 50))
					fmt.Println("IMPORTANT: Coolify Server Setup")
					fmt.Println(strings.Repeat("=", 50))
					fmt.Println()
					fmt.Println("For Coolify to pull from your private registry,")
					fmt.Println("run this command ON YOUR COOLIFY SERVER:")
					fmt.Println()
					fmt.Printf("  echo '%s' | docker login %s -u %s --password-stdin\n", password, registryURL, username)
					fmt.Println()
					fmt.Println("Steps:")
					fmt.Println("  1. SSH into your Coolify server")
					fmt.Println("  2. Run the command above")
					fmt.Println("  3. You should see 'Login Succeeded'")
					fmt.Println(strings.Repeat("=", 50))
				}
			}
		}
	}

	// Save config
	if err := config.SaveGlobal(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Logged in to %s", coolifyURL))
	ui.Dim(fmt.Sprintf("Run '%s' in any project directory to deploy", execName()))

	return nil
}

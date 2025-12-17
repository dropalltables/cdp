package cmd

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show authentication status",
	Long:  "Display current authentication and configuration status.",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if cfg.CoolifyURL == "" || cfg.CoolifyToken == "" {
		ui.Warning("Not authenticated")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s login' to get started", execName()),
		})
		return nil
	}

	ui.Section("Authentication Status")

	// Check connection
	client := api.NewClient(cfg.CoolifyURL, cfg.CoolifyToken)
	var connected bool
	err = ui.WithSpinner("Checking connection", func() error {
		return client.HealthCheck()
	})
	connected = (err == nil)

	ui.Spacer()
	
	// Show connection status
	if connected {
		ui.Success("Connected to Coolify")
	} else {
		ui.Error("Connection failed")
		ui.Dim("Your credentials may be invalid or the server is unreachable")
	}

	ui.Spacer()
	ui.Divider()
	
	// Show configuration
	ui.Section("Configuration")
	ui.KeyValue("Coolify URL", cfg.CoolifyURL)
	
	if cfg.GitHubToken != "" {
		ui.KeyValue("GitHub", ui.SuccessStyle.Render("Configured"))
	} else {
		ui.KeyValue("GitHub", ui.DimStyle.Render("Not configured"))
	}
	
	if cfg.DockerRegistry != nil {
		ui.KeyValue("Docker Registry", cfg.DockerRegistry.URL)
	} else {
		ui.KeyValue("Docker Registry", ui.DimStyle.Render("Not configured"))
	}

	if cfg.DefaultServer != "" {
		ui.KeyValue("Default Server", cfg.DefaultServer)
	}

	return nil
}

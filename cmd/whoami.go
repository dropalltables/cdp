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
	Short: "Show current login status",
	Long:  "Display information about the current Coolify connection.",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.CoolifyURL == "" || cfg.CoolifyToken == "" {
		ui.Warn("Not logged in")
		ui.Dim(fmt.Sprintf("Run '%s login' to authenticate", execName()))
		return nil
	}

	fmt.Println("Current configuration:")
	fmt.Println()
	fmt.Printf("  Coolify URL: %s\n", cfg.CoolifyURL)

	// Check connection
	client := api.NewClient(cfg.CoolifyURL, cfg.CoolifyToken)
	if err := client.HealthCheck(); err != nil {
		ui.Warn("Connection failed - credentials may be invalid")
	} else {
		ui.Success("Connected")
	}

	// Show configured features
	fmt.Println()
	fmt.Println("Configured features:")
	if cfg.GitHubToken != "" {
		fmt.Println("  GitHub: configured")
	} else {
		ui.Dim("  GitHub: not configured")
	}
	if cfg.DockerRegistry != nil {
		fmt.Printf("  Docker registry: %s\n", cfg.DockerRegistry.URL)
	} else {
		ui.Dim("  Docker registry: not configured")
	}

	if cfg.DefaultServer != "" {
		fmt.Printf("  Default server: %s\n", cfg.DefaultServer)
	}

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dropalltables/cdp/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Flags
	prodFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "cdp",
	Short: "Deploy to Coolify with a single command",
	Long: `CDP (Coolify Deploy) is a CLI tool that provides a Vercel-like
deployment experience for Coolify. Just run 'cdp' in any directory
to deploy your application.

No git repository required - cdp handles everything for you.`,
	// Running 'cdp' without subcommand triggers deploy
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploy(prodFlag)
	},
	SilenceUsage: true, // Don't show usage on errors
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&prodFlag, "prod", false, "Deploy to production environment")
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// execName returns the executable name/path as invoked
func execName() string {
	return os.Args[0]
}

// checkLogin ensures the user is logged in
func checkLogin() error {
	if !config.IsLoggedIn() {
		return fmt.Errorf("not logged in. Run '%s login' first", execName())
	}
	return nil
}

// getWorkingDirName returns the name of the current working directory
func getWorkingDirName() string {
	dir, err := os.Getwd()
	if err != nil {
		return "app"
	}
	return filepath.Base(dir)
}

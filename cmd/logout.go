package cmd

import (
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of Coolify",
	Long:  "Remove stored Coolify credentials from this machine.",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	confirm, err := ui.Confirm("Are you sure you want to log out?")
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	if err := config.Clear(); err != nil {
		// Ignore error if file doesn't exist
		ui.Warn("No credentials found")
		return nil
	}

	ui.Success("Logged out successfully")
	return nil
}

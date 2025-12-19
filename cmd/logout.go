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
	ui.Section("Logout")

	confirm, err := ui.Confirm("Remove all stored credentials?")
	if err != nil {
		return err
	}
	if !confirm {
		ui.Dim("Cancelled")
		return nil
	}

	ui.Spacer()
	if err := config.Clear(); err != nil {
		// Ignore error if file doesn't exist
		ui.Warning("No credentials found")
		return nil
	}

	ui.Success("Logged out successfully")
	ui.Spacer()
	ui.Dim("Run 'cdp login' to authenticate again")
	return nil
}

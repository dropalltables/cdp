package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Flags for env command
	previewFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "cdp",
	Short: "Deploy to Coolify with a single command",
	Long: `CDP provides a modern deployment experience for Coolify.
Deploy your applications with a single command - no manual setup required.

Run 'cdp' to deploy, or 'cdp --help' for more commands.`,
	// Running 'cdp' without subcommand triggers deploy
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploy()
	},
	SilenceUsage:  true, // Don't show usage on errors
	SilenceErrors: true, // We handle errors with our UI
}

func init() {
	// Customize help template
	rootCmd.SetHelpFunc(customHelp)
}

// Execute runs the root command
func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		ui.ErrorWithSuggestion(err, "Run 'cdp --help' for usage")
	}
	return err
}

// execName returns the executable name/path as invoked
func execName() string {
	name := filepath.Base(os.Args[0])
	if name == "" {
		return "cdp"
	}
	return name
}

// checkLogin ensures the user is logged in
func checkLogin() error {
	if !config.IsLoggedIn() {
		ui.Error("Not logged in")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s login' to authenticate with Coolify", execName()),
		})
		return fmt.Errorf("authentication required")
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

// customHelp provides a styled help output
func customHelp(cmd *cobra.Command, args []string) {
	ui.Bold("CDP - Coolify Deployment Tool")
	ui.Spacer()
	ui.Print("Deploy applications to Coolify with a single command.")
	ui.Spacer()
	ui.Dim("USAGE")
	ui.Print("  " + cmd.UseLine())
	ui.Spacer()

	if cmd.Long != "" {
		ui.Dim("DESCRIPTION")
		ui.Print("  " + cmd.Long)
		ui.Spacer()
	}

	if len(cmd.Commands()) > 0 {
		ui.Dim("COMMANDS")
		for _, c := range cmd.Commands() {
			if c.Hidden {
				continue
			}
			line := fmt.Sprintf("  %-12s %s", c.Name(), c.Short)
			ui.Print(line)
		}
		ui.Spacer()
	}

	if cmd.HasAvailableFlags() {
		ui.Dim("FLAGS")
		ui.Print(cmd.Flags().FlagUsages())
	}

	ui.Dim("Learn more: https://github.com/dropalltables/cdp")
}

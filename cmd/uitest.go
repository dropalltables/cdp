package cmd

import (
	"fmt"
	"time"

	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var uitestCmd = &cobra.Command{
	Use:    "uitest",
	Short:  "Test all UI components",
	Hidden: true,
	RunE:   runUITest,
}

func init() {
	rootCmd.AddCommand(uitestCmd)
}

func runUITest(cmd *cobra.Command, args []string) error {
	ui.Bold("UI Component Test")
	ui.Dim("Testing all UI components")
	ui.Spacer()

	// Output functions
	ui.Success("Success message")
	ui.Error("Error message")
	ui.Warning("Warning message")
	ui.Info("Info message")
	ui.Dim("Dim message")
	ui.KeyValue("Key", "value")

	ui.Spacer()

	// LogChoice (auto-selection)
	ui.LogChoice("Auto-selected", "Option A")

	// Confirm
	confirmed, err := ui.Confirm("Continue with test")
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(confirmed: %v)", confirmed))

	// Input
	name, err := ui.Input("Your name", "")
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(name: %s)", name))

	// InputWithDefault
	port, err := ui.InputWithDefault("Port", "3000")
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(port: %s)", port))

	// Password
	pass, err := ui.Password("Secret")
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(password length: %d)", len(pass)))

	// Select
	color, err := ui.Select("Pick a color", []string{"Red", "Green", "Blue"})
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(color: %s)", color))

	// SelectWithKeys
	server, err := ui.SelectWithKeys("Pick a server", map[string]string{
		"uuid-1": "Production",
		"uuid-2": "Staging",
	})
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(server uuid: %s)", server))

	// MultiSelect
	features, err := ui.MultiSelect("Pick features", []string{"Auth", "DB", "Cache"})
	if err != nil {
		return err
	}
	ui.Dim(fmt.Sprintf("(features: %v)", features))

	// Task runner with spinner
	ui.Spacer()
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "task-1",
			ActiveName:   "Loading...",
			CompleteName: "Loaded",
			Action: func() error {
				time.Sleep(1 * time.Second)
				return nil
			},
		},
		{
			Name:         "task-2",
			ActiveName:   "Processing...",
			CompleteName: "Processed",
			Action: func() error {
				time.Sleep(1 * time.Second)
				return nil
			},
		},
	})
	if err != nil {
		return err
	}

	ui.Spacer()
	ui.Success("All tests passed")

	return nil
}

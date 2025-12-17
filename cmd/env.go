package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dropalltables/cdp/internal/api"
	"github.com/dropalltables/cdp/internal/config"
	"github.com/dropalltables/cdp/internal/ui"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables",
	Long:  "Manage environment variables for your Coolify application.",
}

var envLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List environment variables",
	RunE:  runEnvLs,
}

var envAddCmd = &cobra.Command{
	Use:   "add KEY=value",
	Short: "Add an environment variable",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvAdd,
}

var envRmCmd = &cobra.Command{
	Use:   "rm KEY",
	Short: "Remove an environment variable",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvRm,
}

var envPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull environment variables to local .env file",
	RunE:  runEnvPull,
}

var envPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local .env file to Coolify",
	RunE:  runEnvPush,
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envLsCmd)
	envCmd.AddCommand(envAddCmd)
	envCmd.AddCommand(envRmCmd)
	envCmd.AddCommand(envPullCmd)
	envCmd.AddCommand(envPushCmd)
}

func getAppUUID() (string, *api.Client, error) {
	if err := checkLogin(); err != nil {
		return "", nil, err
	}

	projectCfg, err := config.LoadProject()
	if err != nil {
		return "", nil, fmt.Errorf("failed to load project config: %w", err)
	}
	if projectCfg == nil {
		return "", nil, fmt.Errorf("not linked to a project. Run '%s' or '%s link' first", execName(), execName())
	}

	env := config.EnvPreview
	if prodFlag {
		env = config.EnvProduction
	}

	appUUID := projectCfg.AppUUIDs[env]
	if appUUID == "" {
		return "", nil, fmt.Errorf("no application found for %s. Deploy first with '%s'", env, execName())
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return "", nil, fmt.Errorf("failed to load config: %w", err)
	}

	client := api.NewClient(globalCfg.CoolifyURL, globalCfg.CoolifyToken)
	return appUUID, client, nil
}

func runEnvLs(cmd *cobra.Command, args []string) error {
	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner("Loading environment variables...")
	spinner.Start()
	envVars, err := client.GetApplicationEnvVars(appUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to get env vars: %w", err)
	}

	if len(envVars) == 0 {
		ui.Dim("No environment variables set")
		return nil
	}

	fmt.Println("Environment variables:")
	for _, env := range envVars {
		value := env.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		fmt.Printf("  %s=%s\n", env.Key, value)
	}

	return nil
}

func runEnvAdd(cmd *cobra.Command, args []string) error {
	parts := strings.SplitN(args[0], "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format. Use: %s env add KEY=value", execName())
	}
	key, value := parts[0], parts[1]

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner("Adding environment variable...")
	spinner.Start()
	_, err = client.CreateApplicationEnvVar(appUUID, key, value, false, false)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to add env var: %w", err)
	}

	ui.Success(fmt.Sprintf("Added %s", key))
	return nil
}

func runEnvRm(cmd *cobra.Command, args []string) error {
	key := args[0]

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	// Find the env var by key
	spinner := ui.NewSpinner("Finding environment variable...")
	spinner.Start()
	envVars, err := client.GetApplicationEnvVars(appUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to get env vars: %w", err)
	}

	var envUUID string
	for _, env := range envVars {
		if env.Key == key {
			envUUID = env.UUID
			break
		}
	}

	if envUUID == "" {
		return fmt.Errorf("environment variable %s not found", key)
	}

	spinner = ui.NewSpinner("Removing environment variable...")
	spinner.Start()
	err = client.DeleteApplicationEnvVar(appUUID, envUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to remove env var: %w", err)
	}

	ui.Success(fmt.Sprintf("Removed %s", key))
	return nil
}

func runEnvPull(cmd *cobra.Command, args []string) error {
	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner("Pulling environment variables...")
	spinner.Start()
	envVars, err := client.GetApplicationEnvVars(appUUID)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to get env vars: %w", err)
	}

	if len(envVars) == 0 {
		ui.Warn("No environment variables to pull")
		return nil
	}

	// Check if .env already exists
	if _, err := os.Stat(".env"); err == nil {
		overwrite, err := ui.Confirm(".env file already exists. Overwrite?")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	file, err := os.Create(".env")
	if err != nil {
		return fmt.Errorf("failed to create .env file: %w", err)
	}
	defer file.Close()

	for _, env := range envVars {
		file.WriteString(fmt.Sprintf("%s=%s\n", env.Key, env.Value))
	}

	ui.Success(fmt.Sprintf("Pulled %d environment variables to .env", len(envVars)))
	return nil
}

func runEnvPush(cmd *cobra.Command, args []string) error {
	// Read .env file
	file, err := os.Open(".env")
	if err != nil {
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	var envVars []struct {
		Key   string
		Value string
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envVars = append(envVars, struct {
			Key   string
			Value string
		}{Key: parts[0], Value: parts[1]})
	}

	if len(envVars) == 0 {
		ui.Warn("No environment variables found in .env")
		return nil
	}

	spinner := ui.NewSpinner("Pushing environment variables...")
	spinner.Start()
	for _, env := range envVars {
		_, err := client.CreateApplicationEnvVar(appUUID, env.Key, env.Value, false, false)
		if err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to push %s: %w", env.Key, err)
		}
	}
	spinner.Stop()

	ui.Success(fmt.Sprintf("Pushed %d environment variables", len(envVars)))
	return nil
}

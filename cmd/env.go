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

	ui.Section("Environment Variables")

	ui.Info("Loading environment variables...")
	allEnvVars, err := client.GetApplicationEnvVars(appUUID)
	if err != nil {
		ui.Error("Failed to load environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}
	
	ui.Success("Loaded environment variables")

	if len(allEnvVars) == 0 {
		ui.Spacer()
		ui.Dim("No environment variables configured")
		ui.NextSteps([]string{
			fmt.Sprintf("Run '%s env add KEY=value' to add variables", execName()),
			fmt.Sprintf("Run '%s env push' to upload from .env file", execName()),
		})
		return nil
	}

	// Build table with environment label
	headers := []string{"Environment", "Key", "Value"}
	rows := [][]string{}

	for _, env := range allEnvVars {
		value := env.Value
		// Mask sensitive values
		if len(value) > 50 {
			value = value[:20] + "..." + value[len(value)-10:]
		}
		if strings.Contains(strings.ToLower(env.Key), "secret") ||
			strings.Contains(strings.ToLower(env.Key), "password") ||
			strings.Contains(strings.ToLower(env.Key), "token") {
			value = "••••••••"
		}

		envLabel := "Production"
		if env.IsPreview {
			envLabel = "Preview"
		}

		rows = append(rows, []string{envLabel, env.Key, value})
	}
	
	ui.Spacer()
	ui.Table(headers, rows)
	ui.Spacer()
	ui.Dim(fmt.Sprintf("Total: %d variables", len(allEnvVars)))

	return nil
}

func runEnvAdd(cmd *cobra.Command, args []string) error {
	parts := strings.SplitN(args[0], "=", 2)
	if len(parts) != 2 {
		ui.Error("Invalid format")
		ui.Spacer()
		ui.Print("Usage: " + ui.CodeStyle.Render(fmt.Sprintf("%s env add KEY=value", execName())))
		return fmt.Errorf("invalid format")
	}
	key, value := parts[0], parts[1]

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	// Set is_preview based on environment
	isPreview := !prodFlag

	ui.Info(fmt.Sprintf("Adding %s...", key))
	_, err = client.CreateApplicationEnvVar(appUUID, key, value, false, isPreview)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to add %s", key))
		return fmt.Errorf("failed to add environment variable: %w", err)
	}
	ui.Success(fmt.Sprintf("Added %s", key))

	ui.NextSteps([]string{
		fmt.Sprintf("Redeploy with '%s' for changes to take effect", execName()),
	})

	return nil
}

func runEnvRm(cmd *cobra.Command, args []string) error {
	key := args[0]

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	// Confirm deletion
	confirmed, err := ui.ConfirmAction("remove environment variable", key)
	if err != nil {
		return err
	}
	if !confirmed {
		ui.Dim("Cancelled")
		return nil
	}

	// Find the env var by key, matching the environment
	isPreview := !prodFlag
	ui.Info("Finding environment variable...")
	envVars, err := client.GetApplicationEnvVars(appUUID)
	if err != nil {
		ui.Error("Failed to fetch environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}

	var envUUID string
	for _, env := range envVars {
		if env.Key == key && env.IsPreview == isPreview {
			envUUID = env.UUID
			break
		}
	}

	if envUUID == "" {
		envName := config.EnvPreview
		if prodFlag {
			envName = config.EnvProduction
		}
		ui.Error(fmt.Sprintf("Variable '%s' not found in %s", key, envName))
		return fmt.Errorf("environment variable '%s' not found in %s", key, envName)
	}

	ui.Info(fmt.Sprintf("Removing %s...", key))
	err = client.DeleteApplicationEnvVar(appUUID, envUUID)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to remove %s", key))
		return fmt.Errorf("failed to remove environment variable: %w", err)
	}
	ui.Success(fmt.Sprintf("Removed %s", key))

	ui.NextSteps([]string{
		fmt.Sprintf("Redeploy with '%s' for changes to take effect", execName()),
	})

	return nil
}

func runEnvPull(cmd *cobra.Command, args []string) error {
	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	envName := config.EnvPreview
	if prodFlag {
		envName = config.EnvProduction
	}

	ui.Section(fmt.Sprintf("Pull Environment Variables - %s", envName))

	ui.Info("Fetching environment variables...")
	envVars, err := client.GetApplicationEnvVars(appUUID)
	if err != nil {
		ui.Error("Failed to fetch environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}
	ui.Success("Fetched environment variables")

	if len(envVars) == 0 {
		ui.Warning("No environment variables to pull")
		return nil
	}

	// Check if .env already exists
	if _, err := os.Stat(".env"); err == nil {
		ui.Spacer()
		overwrite, err := ui.Confirm(".env already exists. Overwrite?")
		if err != nil {
			return err
		}
		if !overwrite {
			ui.Dim("Cancelled")
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

	ui.Spacer()
	ui.Success(fmt.Sprintf("Pulled %d variables to .env", len(envVars)))
	ui.Spacer()
	ui.KeyValue("File", ".env")
	ui.KeyValue("Variables", fmt.Sprintf("%d", len(envVars)))

	return nil
}

func runEnvPush(cmd *cobra.Command, args []string) error {
	// Read .env file
	file, err := os.Open(".env")
	if err != nil {
		ui.Error("Could not open .env file")
		ui.NextSteps([]string{
			"Create a .env file with your environment variables",
			"Format: KEY=value (one per line)",
		})
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	envName := config.EnvPreview
	if prodFlag {
		envName = config.EnvProduction
	}

	ui.Section(fmt.Sprintf("Push Environment Variables - %s", envName))

	var envVars []struct {
		Key   string
		Value string
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			ui.Warning(fmt.Sprintf("Skipping invalid line %d: %s", lineNum, line))
			continue
		}
		envVars = append(envVars, struct {
			Key   string
			Value string
		}{Key: parts[0], Value: parts[1]})
	}

	if len(envVars) == 0 {
		ui.Warning("No valid environment variables found in .env")
		return nil
	}

	ui.Spacer()
	ui.KeyValue("Found", fmt.Sprintf("%d variables", len(envVars)))
	ui.Spacer()

	confirmed, err := ui.Confirm(fmt.Sprintf("Push %d variables to %s?", len(envVars), envName))
	if err != nil {
		return err
	}
	if !confirmed {
		ui.Dim("Cancelled")
		return nil
	}

	ui.Spacer()
	pushed := 0
	failed := 0

	// Set is_preview based on environment
	isPreview := !prodFlag

	for _, env := range envVars {
		ui.Info(fmt.Sprintf("Pushing %s...", env.Key))
		_, err := client.CreateApplicationEnvVar(appUUID, env.Key, env.Value, false, isPreview)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to push %s: %v", env.Key, err))
			failed++
		} else {
			ui.Success(fmt.Sprintf("Pushed %s", env.Key))
			pushed++
		}
	}

	ui.Spacer()
	if failed > 0 {
		ui.Warning(fmt.Sprintf("Pushed %d variables (%d failed)", pushed, failed))
	} else {
		ui.Success(fmt.Sprintf("Pushed %d variables", pushed))
	}

	ui.NextSteps([]string{
		fmt.Sprintf("Run '%s' to redeploy with new variables", execName()),
	})

	return nil
}


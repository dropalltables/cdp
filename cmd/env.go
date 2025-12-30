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

var envResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete all environment variables",
	Long:  "Delete all environment variables for the specified deployment (preview by default, use --prod for production).",
	RunE:  runEnvReset,
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envLsCmd)
	envCmd.AddCommand(envAddCmd)
	envCmd.AddCommand(envRmCmd)
	envCmd.AddCommand(envPullCmd)
	envCmd.AddCommand(envPushCmd)
	envCmd.AddCommand(envResetCmd)

	// Add --prod flag for env commands to target production deployments
	envCmd.PersistentFlags().BoolVar(&prodFlag, "prod", false, "Target production environment (default is preview)")
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

	appUUID := projectCfg.AppUUID
	if appUUID == "" {
		return "", nil, fmt.Errorf("no application found. Deploy first with '%s'", execName())
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

	var allEnvVars []api.EnvVar
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "load-env-vars",
			ActiveName:   "Loading environment variables...",
			CompleteName: "Loaded environment variables",
			Action: func() error {
				var err error
				allEnvVars, err = client.GetApplicationEnvVars(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to load environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}

	if len(allEnvVars) == 0 {
		ui.Warning("No environment variables configured")
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
	ui.Info(fmt.Sprintf("Total: %d variables", len(allEnvVars)))

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

	// Set is_preview based on flag (default is preview, --prod targets production)
	isPreview := !prodFlag

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "add-env-var",
			ActiveName:   fmt.Sprintf("Adding %s...", key),
			CompleteName: fmt.Sprintf("Added %s", key),
			Action: func() error {
				_, err := client.CreateApplicationEnvVar(appUUID, key, value, false, isPreview)
				return err
			},
		},
	})
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to add %s", key))
		return fmt.Errorf("failed to add environment variable: %w", err)
	}

	return nil
}

func runEnvRm(cmd *cobra.Command, args []string) error {
	key := args[0]

	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	// Find the env var by key, matching the deployment type (default is preview, --prod targets production)
	isPreview := !prodFlag
	envVars, err := client.GetApplicationEnvVars(appUUID)
	if err != nil {
		ui.Error("Failed to fetch environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}

	var targetEnv *api.EnvVar
	for _, env := range envVars {
		if env.Key == key && env.IsPreview == isPreview {
			targetEnv = &env
			break
		}
	}

	if targetEnv == nil {
		deploymentType := "preview"
		if prodFlag {
			deploymentType = "production"
		}
		ui.Error(fmt.Sprintf("Variable '%s' not found in %s", key, deploymentType))
		return fmt.Errorf("environment variable '%s' not found in %s", key, deploymentType)
	}

	// Display variable to be deleted
	ui.Warning("This will delete 1 environment variable")
	ui.Spacer()

	headers := []string{"Environment", "Key", "Value"}
	rows := [][]string{}

	value := targetEnv.Value
	// Mask sensitive values
	if len(value) > 50 {
		value = value[:20] + "..." + value[len(value)-10:]
	}
	if strings.Contains(strings.ToLower(targetEnv.Key), "secret") ||
		strings.Contains(strings.ToLower(targetEnv.Key), "password") ||
		strings.Contains(strings.ToLower(targetEnv.Key), "token") {
		value = "••••••••"
	}

	envLabel := "Production"
	if targetEnv.IsPreview {
		envLabel = "Preview"
	}

	rows = append(rows, []string{envLabel, targetEnv.Key, value})

	ui.Table(headers, rows)
	ui.Spacer()

	// Confirm deletion
	confirmed, err := ui.Confirm("Are you sure?")
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Delete variable
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "delete-env-var",
			ActiveName:   "Deleting environment variable...",
			CompleteName: "Deleted 1 variable",
			Action: func() error {
				return client.DeleteApplicationEnvVar(appUUID, targetEnv.UUID)
			},
		},
	})
	if err != nil {
		ui.Error("Failed to delete environment variable")
		return err
	}

	return nil
}

func runEnvPull(cmd *cobra.Command, args []string) error {
	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	var allEnvVars []api.EnvVar
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "fetch-env-vars",
			ActiveName:   "Fetching environment variables...",
			CompleteName: "Fetched environment variables",
			Action: func() error {
				var err error
				allEnvVars, err = client.GetApplicationEnvVars(appUUID)
				return err
			},
		},
	})
	if err != nil {
		ui.Error("Failed to fetch environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}

	// Filter by deployment type (default is preview, --prod targets production)
	isPreview := !prodFlag
	var envVars []api.EnvVar
	for _, env := range allEnvVars {
		if env.IsPreview == isPreview {
			envVars = append(envVars, env)
		}
	}

	if len(envVars) == 0 {
		deploymentType := "preview"
		if prodFlag {
			deploymentType = "production"
		}
		ui.Warning(fmt.Sprintf("No %s environment variables to pull", deploymentType))
		return nil
	}

	// Check if .env already exists
	if _, err := os.Stat(".env"); err == nil {
		ui.Warning(".env file already exists")
		overwrite, err := ui.Confirm("Overwrite?")
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	}

	ui.Spacer()

	headers := []string{"Environment", "Key", "Value"}
	rows := [][]string{}

	for _, env := range envVars {
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

	ui.Table(headers, rows)
	ui.Spacer()

	// Pull variables
	err = ui.RunTasks([]ui.Task{
		{
			Name:         "pull-env-vars",
			ActiveName:   "Pulling environment variables...",
			CompleteName: fmt.Sprintf("Pulled %d variables to .env", len(envVars)),
			Action: func() error {
				file, err := os.Create(".env")
				if err != nil {
					return err
				}
				defer file.Close()

				for _, env := range envVars {
					_, err := file.WriteString(fmt.Sprintf("%s=%s\n", env.Key, env.Value))
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
	if err != nil {
		ui.Error("Failed to pull environment variables")
		return err
	}

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

	// Display variables to be pushed
	ui.Warning(fmt.Sprintf("This will push %d environment variables", len(envVars)))
	ui.Spacer()

	// Determine deployment type for display
	deploymentType := "Preview"
	if prodFlag {
		deploymentType = "Production"
	}

	headers := []string{"Environment", "Key", "Value"}
	rows := [][]string{}

	for _, env := range envVars {
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

		rows = append(rows, []string{deploymentType, env.Key, value})
	}

	ui.Table(headers, rows)
	ui.Spacer()

	// Confirm push
	confirmed, err := ui.Confirm("Are you sure?")
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Push variables
	pushed := 0
	failed := 0

	// Set is_preview based on flag (default is preview, --prod targets production)
	isPreview := !prodFlag

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "push-env-vars",
			ActiveName:   "Pushing environment variables...",
			CompleteName: fmt.Sprintf("Pushed %d variables", len(envVars)),
			Action: func() error {
				for _, env := range envVars {
					_, err := client.CreateApplicationEnvVar(appUUID, env.Key, env.Value, false, isPreview)
					if err != nil {
						failed++
					} else {
						pushed++
					}
				}
				return nil
			},
		},
	})
	if err != nil {
		ui.Error("Failed to push environment variables")
		return err
	}

	if failed > 0 {
		ui.Warning(fmt.Sprintf("%d failed", failed))
	}

	return nil
}

func runEnvReset(cmd *cobra.Command, args []string) error {
	appUUID, client, err := getAppUUID()
	if err != nil {
		return err
	}

	// Determine deployment type
	deploymentType := "preview"
	if prodFlag {
		deploymentType = "production"
	}

	// Fetch all env vars
	envVars, err := client.GetApplicationEnvVars(appUUID)
	if err != nil {
		ui.Error("Failed to fetch environment variables")
		return fmt.Errorf("failed to fetch environment variables: %w", err)
	}

	// Filter by deployment type
	isPreview := !prodFlag
	var varsToDelete []api.EnvVar
	for _, env := range envVars {
		if env.IsPreview == isPreview {
			varsToDelete = append(varsToDelete, env)
		}
	}

	if len(varsToDelete) == 0 {
		ui.Warning(fmt.Sprintf("No %s environment variables to delete", deploymentType))
		return nil
	}

	// Display variables to be deleted
	ui.Warning(fmt.Sprintf("This will delete %d environment variables", len(varsToDelete)))
	ui.Spacer()
	
	headers := []string{"Environment", "Key", "Value"}
	rows := [][]string{}
	
	for _, env := range varsToDelete {
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
	
	ui.Table(headers, rows)
	ui.Spacer()
	
	// Confirm deletion
	confirmed, err := ui.Confirm("Are you sure?")
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Delete all variables
	deleted := 0
	failed := 0

	err = ui.RunTasks([]ui.Task{
		{
			Name:         "delete-env-vars",
			ActiveName:   "Deleting environment variables...",
			CompleteName: fmt.Sprintf("Deleted %d variables", len(varsToDelete)),
			Action: func() error {
				for _, env := range varsToDelete {
					err := client.DeleteApplicationEnvVar(appUUID, env.UUID)
					if err != nil {
						failed++
					} else {
						deleted++
					}
				}
				return nil
			},
		},
	})
	if err != nil {
		ui.Error("Failed to delete environment variables")
		return err
	}

	if failed > 0 {
		ui.Warning(fmt.Sprintf("%d failed", failed))
	}

	return nil
}

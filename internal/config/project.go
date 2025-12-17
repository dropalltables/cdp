package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const projectConfigFile = "cdp.json"

// LoadProject loads the project configuration from the current directory
func LoadProject() (*ProjectConfig, error) {
	return LoadProjectFrom(".")
}

// LoadProjectFrom loads the project configuration from a specific directory
func LoadProjectFrom(dir string) (*ProjectConfig, error) {
	configPath := filepath.Join(dir, projectConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No project config exists
		}
		return nil, err
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveProject saves the project configuration to the current directory
func SaveProject(cfg *ProjectConfig) error {
	return SaveProjectTo(".", cfg)
}

// SaveProjectTo saves the project configuration to a specific directory
func SaveProjectTo(dir string, cfg *ProjectConfig) error {
	configPath := filepath.Join(dir, projectConfigFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// ProjectExists checks if a project config exists in the current directory
func ProjectExists() bool {
	return ProjectExistsIn(".")
}

// ProjectExistsIn checks if a project config exists in a specific directory
func ProjectExistsIn(dir string) bool {
	configPath := filepath.Join(dir, projectConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}

// DeleteProject deletes the project configuration from the current directory
func DeleteProject() error {
	return DeleteProjectFrom(".")
}

// DeleteProjectFrom deletes the project configuration from a specific directory
func DeleteProjectFrom(dir string) error {
	configPath := filepath.Join(dir, projectConfigFile)
	return os.Remove(configPath)
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDir  = ".config/cdp"
	configFile = "config.json"
)

// GetConfigPath returns the path to the global config file
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDir, configFile), nil
}

// LoadGlobal loads the global configuration
func LoadGlobal() (*GlobalConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, err
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveGlobal saves the global configuration
func SaveGlobal(cfg *GlobalConfig) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// IsLoggedIn checks if the user has valid credentials
func IsLoggedIn() bool {
	cfg, err := LoadGlobal()
	if err != nil {
		return false
	}
	return cfg.CoolifyURL != "" && cfg.CoolifyToken != ""
}

// Clear removes the global configuration
func Clear() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}
	return os.Remove(configPath)
}

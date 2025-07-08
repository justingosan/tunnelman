package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir  = ".tunnelman"
	DefaultConfigFile = "config.json"
)

type Config struct {
	CloudflareAPIKey   string `json:"cloudflare_api_key"`
	CloudflareEmail    string `json:"cloudflare_email"`
	TunnelConfigPath   string `json:"tunnel_config_path"`
	AutoRefreshSeconds int    `json:"auto_refresh_seconds"`
	LogLevel           string `json:"log_level"`
}

func DefaultConfig() *Config {
	return &Config{
		TunnelConfigPath:   filepath.Join(getConfigDir(), "tunnels.json"),
		AutoRefreshSeconds: 30,
		LogLevel:           "info",
	}
}

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfigDir
	}
	return filepath.Join(home, DefaultConfigDir)
}

func GetConfigPath() string {
	return filepath.Join(getConfigDir(), DefaultConfigFile)
}

func (c *Config) Save() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := GetConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (s *AppState) SaveToFile(path string) error {
	if path == "" {
		path = s.ConfigPath
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	s.ConfigPath = path
	return nil
}

func LoadAppState(path string) (*AppState, error) {
	if path == "" {
		config, err := LoadConfig()
		if err != nil {
			return nil, err
		}
		path = config.TunnelConfigPath
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		state := NewAppState()
		state.ConfigPath = path
		return state, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	state.ConfigPath = path
	return &state, nil
}

func (s *AppState) Save() error {
	return s.SaveToFile(s.ConfigPath)
}

func (s *AppState) Backup() error {
	if s.ConfigPath == "" {
		return fmt.Errorf("no config path set")
	}

	backupPath := s.ConfigPath + ".backup"
	return s.SaveToFile(backupPath)
}

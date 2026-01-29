package helpers

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

const (
	appName    = "lazyopenconnect"
	configFile = "config.json"
)

func configDir() (string, error) {
	var homeDir string

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return "", err
		}
		homeDir = u.HomeDir
	} else {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}

	return filepath.Join(homeDir, ".config", appName), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

func LoadConfig() (*models.Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return models.NewConfig(), nil
		}
		return nil, err
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.NewConfig(), nil
	}

	valid := make([]models.Connection, 0, len(cfg.Connections))
	for _, c := range cfg.Connections {
		if c.ID != "" && c.Name != "" && c.Host != "" {
			valid = append(valid, c)
		}
	}
	cfg.Connections = valid

	return &cfg, nil
}

func SaveConfig(cfg *models.Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

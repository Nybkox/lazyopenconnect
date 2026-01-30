package helpers

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func GetInstallMethod() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "unknown"
	}

	markerPath := filepath.Join(homeDir, ".config", "lazyopenconnect", ".installed-by")
	file, err := os.Open(markerPath)
	if err != nil {
		return "unknown"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "method=") {
			return strings.TrimPrefix(line, "method=")
		}
	}

	return "unknown"
}

func IsHomebrewInstall() bool {
	return GetInstallMethod() == "homebrew"
}

func GetInstallPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	markerPath := filepath.Join(homeDir, ".config", "lazyopenconnect", ".installed-by")
	file, err := os.Open(markerPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "path=") {
				return strings.TrimPrefix(line, "path=")
			}
		}
	}

	searchPaths := []string{
		filepath.Join(homeDir, ".local", "bin", "lazyopenconnect"),
		"/usr/local/bin/lazyopenconnect",
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

func GetConfigDir() (string, error) {
	return configDir()
}

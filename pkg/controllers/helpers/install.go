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

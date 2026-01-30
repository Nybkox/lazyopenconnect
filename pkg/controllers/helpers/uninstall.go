package helpers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func RemoveBinary() error {
	binaryPath := GetInstallPath()
	if binaryPath == "" {
		return fmt.Errorf("binary not found")
	}

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s", binaryPath)
	}

	if err := os.Remove(binaryPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied removing %s\nRun: sudo lazyopenconnect uninstall", binaryPath)
		}
		return err
	}

	fmt.Printf("Removed %s\n", binaryPath)
	return nil
}

func RemoveKeychain() error {
	cfg, err := LoadConfig()
	if err != nil {
		return nil
	}

	removed := 0
	for _, conn := range cfg.Connections {
		if conn.HasPassword {
			if err := DeletePassword(conn.ID); err == nil {
				removed++
			}
		}
	}

	if removed > 0 {
		fmt.Printf("Removed %d keychain entry(s)\n", removed)
	}
	return nil
}

func RemoveConfigDir() error {
	dir, err := GetConfigDir()
	if err != nil {
		return nil
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	fmt.Printf("Removed %s\n", dir)
	return nil
}

func RemovePathExport() error {
	shellConfig := detectShellConfig()
	if shellConfig == "" {
		return nil
	}

	if _, err := os.Stat(shellConfig); os.IsNotExist(err) {
		return nil
	}

	pathLine := `export PATH="$HOME/.local/bin:$PATH"`

	content, err := os.ReadFile(shellConfig)
	if err != nil {
		return nil
	}

	if !strings.Contains(string(content), pathLine) {
		return nil
	}

	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != pathLine {
			newLines = append(newLines, line)
		}
	}

	if err := os.WriteFile(shellConfig, []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
		return err
	}

	fmt.Printf("Removed PATH export from %s\n", shellConfig)
	return nil
}

func detectShellConfig() string {
	shell := filepath.Base(os.Getenv("SHELL"))
	homeDir := getHomeDir()
	if homeDir == "" {
		return ""
	}

	switch shell {
	case "zsh":
		return filepath.Join(homeDir, ".zshrc")
	case "bash":
		if runtime.GOOS == "darwin" {
			return filepath.Join(homeDir, ".bash_profile")
		}
		return filepath.Join(homeDir, ".bashrc")
	default:
		return filepath.Join(homeDir, ".profile")
	}
}

func getHomeDir() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		homeDir := "/home/" + sudoUser
		if runtime.GOOS == "darwin" {
			homeDir = "/Users/" + sudoUser
		}
		if _, err := os.Stat(homeDir); err == nil {
			return homeDir
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return homeDir
}

package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// DefaultExportPath returns the default path for exporting logs.
func DefaultExportPath() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join("/tmp", "lazyopenconnect", "exported-logs-"+timestamp+".log")
}

// ExportLogs writes output lines to the specified path, stripping ANSI codes.
func ExportLogs(path string, lines []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var cleaned []string
	for _, line := range lines {
		cleaned = append(cleaned, StripANSI(line))
	}

	content := strings.Join(cleaned, "\n")
	return os.WriteFile(path, []byte(content), 0o644)
}

// CopyLogsToClipboard copies output lines to the system clipboard, stripping ANSI codes.
func CopyLogsToClipboard(lines []string) error {
	var cleaned []string
	for _, line := range lines {
		cleaned = append(cleaned, StripANSI(line))
	}
	content := strings.Join(cleaned, "\n")
	return clipboard.WriteAll(content)
}

func VpnLogPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vpn.log"), nil
}

func ReadVpnLog() (string, error) {
	path, err := VpnLogPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func CopyVpnLogToPath(destPath string, stripANSI bool) error {
	content, err := ReadVpnLog()
	if err != nil {
		return err
	}

	if stripANSI {
		content = StripANSI(content)
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(destPath, []byte(content), 0o644)
}

func CopyVpnLogToClipboard() error {
	content, err := ReadVpnLog()
	if err != nil {
		return err
	}
	return clipboard.WriteAll(StripANSI(content))
}

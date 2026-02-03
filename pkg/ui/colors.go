package ui

import "github.com/charmbracelet/lipgloss"

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

var (
	ColorPrimary    = lipgloss.Color("212")
	ColorSecondary  = lipgloss.Color("240")
	ColorSuccess    = lipgloss.Color("82")
	ColorWarning    = lipgloss.Color("214")
	ColorDanger     = lipgloss.Color("196")
	ColorMuted      = lipgloss.Color("245")
	ColorDim        = lipgloss.Color("238")
	ColorBackground = lipgloss.Color("235")
	ColorForeground = lipgloss.Color("252")
	ColorCommand    = lipgloss.Color("36")
)

func LogError(msg string) string   { return ansiRed + msg + ansiReset }
func LogSuccess(msg string) string { return ansiGreen + msg + ansiReset }
func LogWarning(msg string) string { return ansiYellow + msg + ansiReset }
func LogCommand(cmd string) string { return ansiCyan + "$ " + cmd + ansiReset }
func LogOK(msg string) string      { return ansiGreen + "  " + msg + ansiReset }
func LogFail(msg string) string    { return ansiRed + "  " + msg + ansiReset }

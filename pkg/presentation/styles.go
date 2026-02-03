package presentation

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

var (
	borderStyle = lipgloss.RoundedBorder()

	PaneStyle = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(ui.ColorSecondary)

	PaneFocusedStyle = lipgloss.NewStyle().
				Border(borderStyle).
				BorderForeground(ui.ColorPrimary)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorPrimary)

	StatusBarStyle = lipgloss.NewStyle().
			Background(ui.ColorBackground).
			Foreground(ui.ColorForeground).
			Padding(0, 1)

	ConnectionItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	ConnectionItemSelectedStyle = lipgloss.NewStyle().
					PaddingLeft(2).
					Bold(true).
					Foreground(ui.ColorPrimary)

	ConnectionDetailStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(ui.ColorMuted)

	StatusConnected    = lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render("●")
	StatusConnecting   = lipgloss.NewStyle().Foreground(ui.ColorWarning)
	StatusDisconnected = lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(" ")

	InputStyle = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(ui.ColorSecondary).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(borderStyle).
				BorderForeground(ui.ColorPrimary).
				Padding(0, 1)

	HelpKeyStyle  = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	HelpDescStyle = lipgloss.NewStyle().Foreground(ui.ColorMuted)

	DimStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)

	FormOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.ColorPrimary).
				Padding(1, 2).
				Background(ui.ColorBackground)

	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(ui.ColorMuted)

	SuccessStyle = lipgloss.NewStyle().Foreground(ui.ColorSuccess)
	WarningStyle = lipgloss.NewStyle().Foreground(ui.ColorWarning)
	MutedStyle   = lipgloss.NewStyle().Foreground(ui.ColorMuted)
)

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

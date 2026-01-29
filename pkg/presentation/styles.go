package presentation

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary    = lipgloss.Color("212")
	colorSecondary  = lipgloss.Color("240")
	colorSuccess    = lipgloss.Color("82")
	colorWarning    = lipgloss.Color("214")
	colorDanger     = lipgloss.Color("196")
	colorMuted      = lipgloss.Color("245")
	colorDim        = lipgloss.Color("238")
	colorBackground = lipgloss.Color("235")
	colorForeground = lipgloss.Color("252")

	borderStyle = lipgloss.RoundedBorder()

	PaneStyle = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(colorSecondary)

	PaneFocusedStyle = lipgloss.NewStyle().
				Border(borderStyle).
				BorderForeground(colorPrimary)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	StatusBarStyle = lipgloss.NewStyle().
			Background(colorBackground).
			Foreground(colorForeground).
			Padding(0, 1)

	ConnectionItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	ConnectionItemSelectedStyle = lipgloss.NewStyle().
					PaddingLeft(2).
					Bold(true).
					Foreground(colorPrimary)

	ConnectionDetailStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(colorMuted)

	StatusConnected    = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	StatusConnecting   = lipgloss.NewStyle().Foreground(colorWarning)
	StatusDisconnected = lipgloss.NewStyle().Foreground(colorMuted).Render(" ")

	InputStyle = lipgloss.NewStyle().
			Border(borderStyle).
			BorderForeground(colorSecondary).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(borderStyle).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	HelpKeyStyle  = lipgloss.NewStyle().Foreground(colorPrimary)
	HelpDescStyle = lipgloss.NewStyle().Foreground(colorMuted)

	DimStyle = lipgloss.NewStyle().Foreground(colorDim)

	FormOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2).
				Background(colorBackground)

	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(colorDim)
	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(colorMuted)

	SuccessStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	WarningStyle = lipgloss.NewStyle().Foreground(colorWarning)
	MutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
)

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

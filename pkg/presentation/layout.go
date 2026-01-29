package presentation

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Nybkox/lazyopenconnect/pkg/app"
	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func Render(state *app.State, spinnerFrame int) string {
	if state.Width == 0 || state.Height == 0 {
		return "Loading..."
	}

	if state.Width < 80 || state.Height < 15 {
		return fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 80x15", state.Width, state.Height)
	}

	leftWidth := state.Width / 2
	rightWidth := state.Width - leftWidth
	totalHeight := state.Height - 1

	statusHeight := 4
	settingsHeight := 5
	connectionsHeight := totalHeight - statusHeight - settingsHeight

	inputHeight := 5
	outputHeight := totalHeight - inputHeight

	statusPane := renderPane("Status", "1", renderStatusContent(state), leftWidth, statusHeight, state.FocusedPane == app.PaneStatus, state.ActiveForm != nil)
	connectionsPane := renderPane("Connections", "2", renderConnectionsContent(state, connectionsHeight-3, leftWidth-2, spinnerFrame), leftWidth, connectionsHeight, state.FocusedPane == app.PaneConnections, state.ActiveForm != nil)

	settingsTitle := "Settings"
	if state.ResetPending {
		settingsTitle = "Settings " + WarningStyle.Render("[r to confirm reset]")
	}
	settingsPane := renderPane(settingsTitle, "3", renderSettingsContent(state), leftWidth, settingsHeight, state.FocusedPane == app.PaneSettings, state.ActiveForm != nil)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, statusPane, connectionsPane, settingsPane)

	outputPane := renderPane("Output", "4", renderOutputContent(state, outputHeight-3), rightWidth, outputHeight, state.FocusedPane == app.PaneOutput, state.ActiveForm != nil)

	inputTitle := "Input"
	if state.IsPasswordPrompt {
		inputTitle = "Password " + MutedStyle.Render("◉")
	}
	inputPane := renderPane(inputTitle, "5", renderInputContent(state), rightWidth, inputHeight, state.FocusedPane == app.PaneInput, state.ActiveForm != nil)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left, outputPane, inputPane)

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)

	if state.ActiveForm != nil {
		main = overlayForm(main, state.ActiveForm.View(), state.Width, totalHeight)
	}

	statusBar := renderStatusBar(state, state.Width)

	return lipgloss.JoinVertical(lipgloss.Left, main, statusBar)
}

func renderPane(title, key, content string, width, height int, focused, formActive bool) string {
	innerWidth := width - 2
	innerHeight := height - 2

	style := PaneStyle.Width(innerWidth).Height(innerHeight)
	if focused && !formActive {
		style = PaneFocusedStyle.Width(innerWidth).Height(innerHeight)
	}

	titleLine := TitleStyle.Render(title) + " " + HelpKeyStyle.Render("["+key+"]")

	inner := titleLine + "\n" + content

	return style.Render(inner)
}

func renderStatusContent(state *app.State) string {
	switch state.Status {
	case app.StatusConnected:
		conn := state.ActiveConnection()
		name := ""
		if conn != nil {
			name = conn.Name + " "
		}
		return fmt.Sprintf("%s Connected %s(%s) pid %d",
			SuccessStyle.Render("●"), name, state.IP, state.PID)
	case app.StatusExternal:
		// Try to match external connection to saved connection
		name := ""
		if conn := state.MatchConnectionByHost(state.ExternalHost); conn != nil {
			name = conn.Name + " "
		} else if state.ExternalHost != "" {
			name = state.ExternalHost + " "
		}
		return fmt.Sprintf("%s External %s(pid %d)",
			WarningStyle.Render("●"), name, state.PID)
	case app.StatusConnecting:
		frame := SpinnerFrames[spinnerFrame%len(SpinnerFrames)]
		return fmt.Sprintf("%s Connecting...", WarningStyle.Render(frame))
	case app.StatusPrompting:
		return fmt.Sprintf("%s Awaiting input...", WarningStyle.Render("○"))
	case app.StatusReconnecting:
		return fmt.Sprintf("%s Reconnecting... (attempt %d/5)",
			WarningStyle.Render("◐"), state.ReconnectAttempts)
	default:
		return fmt.Sprintf("%s Disconnected", MutedStyle.Render("○"))
	}
}

var spinnerFrame int

func renderConnectionsContent(state *app.State, maxLines int, paneWidth int, frame int) string {
	spinnerFrame = frame

	connCount := len(state.Config.Connections)
	if connCount == 0 {
		return ConnectionDetailStyle.Render("No connections.\nPress [n] to add one.")
	}

	visible := state.ConnectionsVisible
	if visible < 1 {
		visible = maxLines / 2
	}
	visible = max(visible, 1)

	scroll := state.ConnectionsScroll
	maxScroll := max(connCount-visible, 0)
	scroll = max(min(scroll, maxScroll), 0)

	endIdx := min(scroll+visible, connCount)

	var lines []string
	for i := scroll; i < endIdx; i++ {
		conn := state.Config.Connections[i]
		lines = append(lines, renderConnectionItem(state, &conn, i, frame))
	}

	content := strings.Join(lines, "\n")

	if connCount > visible {
		content = addScrollbar(content, maxLines, paneWidth, scroll, connCount, visible)
	}

	return content
}

func addScrollbar(content string, height, paneWidth, scroll, total, visible int) string {
	lines := strings.Split(content, "\n")

	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	targetWidth := paneWidth - 2

	trackHeight := max(height, 1)

	thumbSize := max((visible*trackHeight)/total, 1)

	scrollRange := max(total-visible, 1)
	thumbPos := (scroll * (trackHeight - thumbSize)) / scrollRange

	track := ScrollbarTrackStyle.Render("│")
	thumb := ScrollbarThumbStyle.Render("█")

	for i := 0; i < len(lines); i++ {
		char := track
		if i >= thumbPos && i < thumbPos+thumbSize {
			char = thumb
		}
		lineWidth := lipgloss.Width(lines[i])
		padLen := max(targetWidth-lineWidth, 0)
		padding := strings.Repeat(" ", padLen)
		lines[i] = lines[i] + padding + " " + char
	}

	return strings.Join(lines, "\n")
}

func renderConnectionItem(state *app.State, conn *models.Connection, idx int, spinnerFrame int) string {
	isSelected := idx == state.Selected
	isActive := conn.ID == state.ActiveConnID
	isExternal := state.Status == app.StatusExternal && conn.Host == state.ExternalHost

	var indicator string
	if isActive {
		switch state.Status {
		case app.StatusConnected:
			indicator = StatusConnected
		case app.StatusConnecting:
			indicator = StatusConnecting.Render(SpinnerFrames[spinnerFrame%len(SpinnerFrames)])
		default:
			indicator = "  "
		}
	} else if isExternal {
		// Yellow indicator for external-matched connection
		indicator = WarningStyle.Render("●") + " "
	} else {
		indicator = "  "
	}

	marker := "  "
	style := ConnectionItemStyle
	if isSelected {
		marker = "› "
		style = ConnectionItemSelectedStyle
	}

	name := style.Render(marker + conn.Name)
	detail := ConnectionDetailStyle.Render(fmt.Sprintf("  %s · %s", conn.Protocol, conn.Host))

	return fmt.Sprintf("%s %s\n%s", name, indicator, detail)
}

func renderSettingsContent(state *app.State) string {
	dns := state.Config.Settings.DNS
	if dns == "" {
		dns = "(system)"
	}
	reconnect := "off"
	if state.Config.Settings.Reconnect {
		reconnect = "on"
	}

	return fmt.Sprintf("%s %s  %s %s",
		MutedStyle.Render("DNS:"), dns,
		MutedStyle.Render("Reconnect:"), reconnect,
	)
}

func renderOutputContent(state *app.State, height int) string {
	if state.OutputView == "" {
		return MutedStyle.Render("No output yet.")
	}
	return state.OutputView
}

func renderInputContent(state *app.State) string {
	return state.InputView
}

func renderStatusBar(state *app.State, width int) string {
	var help string
	if state.ActiveForm != nil {
		help = "[tab] next  [enter] save  [esc] cancel"
	} else if state.ReconnectCountdown > 0 {
		help = "[esc] cancel reconnect"
	} else {
		switch state.FocusedPane {
		case app.PaneStatus:
			switch state.Status {
			case app.StatusReconnecting:
				help = "[d] cancel reconnect  [1-5] switch pane  [tab] cycle"
			case app.StatusExternal, app.StatusConnected:
				help = "[d] disconnect  [1-5] switch pane  [tab] cycle"
			default:
				help = "[1-5] switch pane  [tab] cycle"
			}
		case app.PaneConnections:
			if state.Status == app.StatusExternal {
				help = "[j/k] nav  [g/G] top/end  [d] disconnect  [n] new  [e] edit  [x] del"
			} else {
				help = "[j/k] nav  [g/G] top/end  [enter] connect  [d] disc  [n] new  [e] edit  [x] del"
			}
		case app.PaneSettings:
			if state.ResetPending {
				help = "[r] confirm reset  [any] cancel"
			} else {
				help = "[enter] edit settings  [r][r] reset defaults"
			}
		case app.PaneOutput:
			help = "[j/k] scroll  [ctrl+d/u] page  [g/G] top/bottom"
		case app.PaneInput:
			if state.Status == app.StatusConnected || state.Status == app.StatusExternal || state.Status == app.StatusReconnecting {
				help = "[enter] submit  [ctrl+d] disconnect  [ctrl+c] quit"
			} else {
				help = "[enter] submit  [ctrl+c] quit"
			}
		}
	}

	return StatusBarStyle.Width(width).Render(help)
}

func overlayForm(base, form string, width, height int) string {
	styledForm := FormOverlayStyle.Render(form)

	// Dim the base content
	dimmed := dimContent(base, height)

	// Composite the form over the dimmed base
	return compositeOverlay(dimmed, styledForm, width, height)
}

func dimContent(content string, height int) string {
	lines := strings.Split(content, "\n")

	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	var result []string
	for _, line := range lines {
		plain := stripAnsi(line)
		result = append(result, DimStyle.Render(plain))
	}

	return strings.Join(result, "\n")
}

func stripAnsi(s string) string {
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

func compositeOverlay(base, overlay string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := len(overlayLines)

	startY := max((height-overlayHeight)/2, 0)
	startX := max((width-overlayWidth)/2, 0)

	for i, overlayLine := range overlayLines {
		baseY := startY + i
		if baseY >= len(baseLines) {
			break
		}

		basePlain := stripAnsi(baseLines[baseY])
		baseRunes := []rune(basePlain)

		for len(baseRunes) < width {
			baseRunes = append(baseRunes, ' ')
		}

		overlayRunes := []rune(overlayLine)
		overlayPlainWidth := lipgloss.Width(overlayLine)

		left := DimStyle.Render(string(baseRunes[:startX]))

		rightStart := startX + overlayPlainWidth
		var right string
		if rightStart < len(baseRunes) {
			right = DimStyle.Render(string(baseRunes[rightStart:]))
		}

		baseLines[baseY] = left + string(overlayRunes) + right
	}

	return strings.Join(baseLines[:height], "\n")
}

package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
)

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if a.State.ActiveForm != nil {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(keyMsg, a.Keys.Cancel) {
				a.State.ActiveForm = nil
				a.State.FormKind = FormNone
				a.State.FormData = nil
				return a, nil
			}
			if key.Matches(keyMsg, a.Keys.Quit) {
				return a, tea.Quit
			}
		}
		return a.updateForm(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleWindowSize(msg)

	case spinnerTickMsg:
		return a.handleSpinnerTick()

	case helpers.VPNStdinReady:
		return a.handleVPNStdinReady(msg)

	case helpers.VPNLogMsg:
		return a.handleVPNLog(msg)

	case helpers.VPNPromptMsg:
		return a.handleVPNPrompt(msg)

	case helpers.VPNConnectedMsg:
		return a.handleVPNConnected(msg)

	case connectionTimeoutMsg:
		return a.handleConnectionTimeout(msg)

	case reconnectTickMsg:
		return a.handleReconnectTick()

	case helpers.VPNDisconnectedMsg:
		return a.handleVPNDisconnected()

	case helpers.VPNCleanupStepMsg:
		return a.handleVPNCleanupStep(msg)

	case helpers.VPNCleanupDoneMsg:
		return a.handleVPNCleanupDone()

	case helpers.VPNErrorMsg:
		return a.handleVPNError(msg)

	case externalCheckTickMsg:
		return a.handleExternalCheckTick()

	case helpers.VPNExternalDetectedMsg:
		return a.handleVPNExternalDetected(msg)

	case helpers.VPNNoExternalMsg:
		return a.handleVPNNoExternal()

	case helpers.VPNProcessAliveMsg:
		return a, nil

	case helpers.VPNProcessDiedMsg:
		return a.handleVPNProcessDied()

	case resetTimeoutMsg:
		return a.handleResetTimeout()

	case UpdateCheckMsg:
		return a.handleUpdateCheck(msg)

	case UpdatePerformedMsg:
		return a.handleUpdatePerformed(msg)

	case tea.KeyMsg:
		return a.handleKeyMsg(msg)
	}

	a.viewport.SetContent(a.renderOutput())

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

func (a *App) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.State.Width = msg.Width
	a.State.Height = msg.Height
	rightWidth := msg.Width - msg.Width/2
	totalHeight := msg.Height - 1
	inputHeight := 5
	outputHeight := totalHeight - inputHeight
	a.viewport.Width = rightWidth - 4
	a.viewport.Height = outputHeight - 3
	statusHeight := 4
	settingsHeight := 5
	connectionsHeight := totalHeight - statusHeight - settingsHeight
	maxLines := connectionsHeight - 3
	a.State.ConnectionsVisible = maxLines / 2
	if a.State.ConnectionsVisible < 1 {
		a.State.ConnectionsVisible = 1
	}
	return a, nil
}

func (a *App) handleSpinnerTick() (tea.Model, tea.Cmd) {
	a.spinnerFrame++
	if a.State.Status == StatusConnecting {
		return a, spinnerTick()
	}
	return a, nil
}

func (a *App) handleVPNStdinReady(msg helpers.VPNStdinReady) (tea.Model, tea.Cmd) {
	a.State.Stdin = msg.Stdin
	a.State.PID = msg.PID
	return a, tea.Batch(
		helpers.WaitForLog(),
		helpers.WaitForPrompt(),
		helpers.WaitForConnected(),
	)
}

func (a *App) handleVPNLog(msg helpers.VPNLogMsg) (tea.Model, tea.Cmd) {
	line := string(msg)
	a.State.OutputLines = append(a.State.OutputLines, line)
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()
	return a, helpers.WaitForLog()
}

func (a *App) handleVPNPrompt(msg helpers.VPNPromptMsg) (tea.Model, tea.Cmd) {
	a.State.Status = StatusPrompting
	a.State.FocusedPane = PaneInput
	a.State.IsPasswordPrompt = msg.IsPassword
	if msg.IsPassword {
		a.input.EchoMode = textinput.EchoPassword
	} else {
		a.input.EchoMode = textinput.EchoNormal
	}
	return a, tea.Batch(a.input.Focus(), helpers.WaitForPrompt())
}

func (a *App) handleVPNConnected(msg helpers.VPNConnectedMsg) (tea.Model, tea.Cmd) {
	a.State.Status = StatusConnected
	a.State.ReconnectAttempts = 0 // Reset on successful connect
	a.State.ReconnectConnID = ""
	if msg.IP != "" {
		a.State.IP = msg.IP
	}
	if msg.PID != 0 {
		a.State.PID = msg.PID
	}
	return a, nil
}

func (a *App) handleConnectionTimeout(connectionTimeoutMsg) (tea.Model, tea.Cmd) {
	// Ignore if not connecting (already connected or disconnected)
	if a.State.Status != StatusConnecting {
		return a, nil
	}

	// Timeout with no success pattern = failure
	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[31m[Connection timeout - no success indicator received]\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.Stdin != nil {
		a.State.Stdin.Close()
	}

	// Reconnect if setting enabled
	if a.State.Config.Settings.Reconnect && a.State.ActiveConnID != "" {
		// Check max attempts before trying again
		if a.State.ReconnectAttempts >= maxReconnectAttempts {
			return a.reconnectFailed()
		}
		a.State.Status = StatusReconnecting
		a.State.ReconnectConnID = a.State.ActiveConnID
		return a.attemptReconnect()
	}

	// Otherwise just disconnect and cleanup
	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	return a, tea.Batch(
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) handleReconnectTick() (tea.Model, tea.Cmd) {
	if a.State.Status != StatusReconnecting || a.State.ReconnectConnID == "" {
		return a, nil
	}
	return a.attemptReconnect()
}

func (a *App) handleVPNDisconnected() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusQuitting {
		return a, nil
	}

	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	a.State.Stdin = nil
	a.State.ReconnectAttempts = 0
	a.State.ReconnectConnID = ""
	a.State.OutputLines = append(a.State.OutputLines, "--- Disconnected ---")

	return a, tea.Batch(
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) handleVPNCleanupStep(msg helpers.VPNCleanupStepMsg) (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines, string(msg))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()
	return a, helpers.WaitForCleanupStep()
}

func (a *App) handleVPNCleanupDone() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines, "--- Cleanup complete ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.Status == StatusQuitting {
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) handleVPNError(msg helpers.VPNErrorMsg) (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[31mError: "+msg.Error()+"\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.ReconnectConnID != "" && a.State.ReconnectAttempts > 0 {
		if a.State.ReconnectAttempts >= maxReconnectAttempts {
			return a.reconnectFailed()
		}
		a.State.Status = StatusReconnecting
		a.State.OutputLines = append(a.State.OutputLines,
			fmt.Sprintf("\x1b[33mRetrying in %ds...\x1b[0m", int(reconnectDelay.Seconds())))
		a.viewport.SetContent(a.renderOutput())
		return a, scheduleReconnectTick()
	}

	a.State.Status = StatusDisconnected
	return a, nil
}

func (a *App) handleExternalCheckTick() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if a.State.Status == StatusDisconnected || a.State.Status == StatusExternal {
		cmds = append(cmds, helpers.CheckExternalVPN())
	}
	if a.State.Status == StatusConnected && a.State.PID > 0 {
		cmds = append(cmds, helpers.CheckProcessAlive(a.State.PID))
	}
	cmds = append(cmds, a.scheduleExternalCheck())
	return a, tea.Batch(cmds...)
}

func (a *App) handleVPNExternalDetected(msg helpers.VPNExternalDetectedMsg) (tea.Model, tea.Cmd) {
	if a.State.Status == StatusDisconnected {
		a.State.Status = StatusExternal
		a.State.PID = msg.PID
		a.State.ExternalHost = msg.Host

		logMsg := fmt.Sprintf("Detected external openconnect (pid %d)", msg.PID)
		if conn := a.State.MatchConnectionByHost(msg.Host); conn != nil {
			logMsg = fmt.Sprintf("Detected external openconnect: %s (pid %d)", conn.Name, msg.PID)
		} else if msg.Host != "" {
			logMsg = fmt.Sprintf("Detected external openconnect: %s (pid %d)", msg.Host, msg.PID)
		}
		a.State.OutputLines = append(a.State.OutputLines, logMsg)
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, nil
}

func (a *App) handleVPNNoExternal() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusExternal {
		a.State.Status = StatusDisconnected
		a.State.PID = 0
		a.State.ExternalHost = ""
		a.State.OutputLines = append(a.State.OutputLines, "External openconnect terminated")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, nil
}

func (a *App) handleVPNProcessDied() (tea.Model, tea.Cmd) {
	if a.State.Status != StatusConnected {
		return a, nil
	}

	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[31m--- Connection lost ---\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.Stdin != nil {
		a.State.Stdin.Close()
		a.State.Stdin = nil
	}

	if a.State.Config.Settings.Reconnect && a.State.ActiveConnID != "" {
		a.State.Status = StatusReconnecting
		a.State.ReconnectAttempts = 0
		a.State.ReconnectConnID = a.State.ActiveConnID
		return a.attemptReconnect()
	}

	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.PID = 0
	a.State.IP = ""
	return a, tea.Batch(
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) handleResetTimeout() (tea.Model, tea.Cmd) {
	if a.State.ResetPending {
		a.State.ResetPending = false
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[33m[Reset cancelled - timeout]\x1b[0m")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, nil
}

func (a *App) handleQuit() (tea.Model, tea.Cmd) {
	needsCleanup := (a.State.Status == StatusConnected || a.State.Status == StatusExternal) &&
		a.State.Config.Settings.AutoCleanup

	if !needsCleanup {
		return a, tea.Quit
	}

	a.State.Status = StatusQuitting
	a.State.OutputLines = append(a.State.OutputLines, "--- Quitting, running cleanup... ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.Stdin != nil {
		a.State.Stdin.Close()
		a.State.Stdin = nil
	}

	return a, tea.Batch(
		helpers.DisconnectVPN(),
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.State.ReconnectCountdown > 0 && key.Matches(msg, a.Keys.Cancel) {
		a.State.ReconnectCountdown = 0
		a.State.ReconnectConnID = ""
		a.State.ReconnectAttempts = 0
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[33m[Reconnect cancelled]\x1b[0m")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	}

	switch {
	case key.Matches(msg, a.Keys.Quit):
		return a.handleQuit()

	case key.Matches(msg, a.Keys.TabFocus):
		a.cycleFocus()
		if a.State.FocusedPane == PaneInput {
			return a, a.input.Focus()
		}
		a.input.Blur()
		return a, nil
	}

	if a.State.FocusedPane != PaneInput {
		switch {
		case key.Matches(msg, a.Keys.FocusPane1):
			a.State.FocusedPane = PaneStatus
			a.input.Blur()
			return a, nil

		case key.Matches(msg, a.Keys.FocusPane2):
			a.State.FocusedPane = PaneConnections
			a.input.Blur()
			return a, nil

		case key.Matches(msg, a.Keys.FocusPane3):
			a.State.FocusedPane = PaneSettings
			a.input.Blur()
			return a, nil

		case key.Matches(msg, a.Keys.FocusPane4):
			a.State.FocusedPane = PaneOutput
			a.input.Blur()
			return a, nil

		case key.Matches(msg, a.Keys.FocusPane5):
			a.State.FocusedPane = PaneInput
			return a, a.input.Focus()
		}
	}

	switch a.State.FocusedPane {
	case PaneStatus:
		if key.Matches(msg, a.Keys.Disconnect) {
			if a.State.Status == StatusExternal || a.State.Status == StatusConnected || a.State.Status == StatusReconnecting {
				return a.disconnect()
			}
		}
		return a, nil
	case PaneConnections:
		return a.updateConnections(msg)
	case PaneSettings:
		return a.updateSettings(msg)
	case PaneOutput:
		return a.updateOutput(msg)
	case PaneInput:
		return a.updateInput(msg)
	}

	return a, nil
}

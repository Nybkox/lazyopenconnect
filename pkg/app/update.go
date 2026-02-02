package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
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

	case DaemonMsg:
		return a.handleDaemonMsg(msg)

	case DaemonDisconnectedMsg:
		return a.handleDaemonDisconnected()

	case helpers.VPNCleanupStepMsg:
		return a.handleVPNCleanupStep(msg)

	case helpers.VPNCleanupDoneMsg:
		return a.handleVPNCleanupDone()

	case reconnectTickMsg:
		return a.handleReconnectTick()

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

func (a *App) handleDaemonMsg(msg DaemonMsg) (tea.Model, tea.Cmd) {
	msgType, _ := msg.Raw["type"].(string)

	switch msgType {
	case "hello_response":
		return a.handleHelloResponse(msg.Raw)
	case "state":
		return a.handleDaemonState(msg.Raw)
	case "log":
		return a.handleDaemonLog(msg.Raw)
	case "prompt":
		return a.handleDaemonPrompt(msg.Raw)
	case "connected":
		return a.handleDaemonConnected(msg.Raw)
	case "disconnected":
		return a.handleDaemonDisconnectedEvent()
	case "error":
		return a.handleDaemonError(msg.Raw)
	case "kicked":
		return a.handleKicked()
	case "cleanup_step":
		return a.handleDaemonCleanupStep(msg.Raw)
	case "cleanup_done":
		return a.handleDaemonCleanupDone()
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleHelloResponse(msg map[string]any) (tea.Model, tea.Cmd) {
	compatible, _ := msg["compatible"].(bool)
	if !compatible {
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[31mDaemon version mismatch. Please restart the daemon.\x1b[0m")
		a.viewport.SetContent(a.renderOutput())
		return a, tea.Quit
	}

	a.SendToDaemon(daemon.ConfigUpdateCmd{
		Type:   "config_update",
		Config: *a.State.Config,
	})

	a.SendToDaemon(daemon.GetStateCmd{Type: "get_state"})

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonState(msg map[string]any) (tea.Model, tea.Cmd) {
	if status, ok := msg["status"].(float64); ok {
		a.State.Status = ConnStatus(int(status))
	}
	if connID, ok := msg["active_conn_id"].(string); ok {
		a.State.ActiveConnID = connID
	}
	if ip, ok := msg["ip"].(string); ok {
		a.State.IP = ip
	}
	if pid, ok := msg["pid"].(float64); ok {
		a.State.PID = int(pid)
	}
	if history, ok := msg["log_history"].([]any); ok {
		a.State.OutputLines = make([]string, 0, len(history))
		for _, line := range history {
			if s, ok := line.(string); ok {
				a.State.OutputLines = append(a.State.OutputLines, s)
			}
		}
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonLog(msg map[string]any) (tea.Model, tea.Cmd) {
	if line, ok := msg["line"].(string); ok {
		a.State.OutputLines = append(a.State.OutputLines, line)
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonPrompt(msg map[string]any) (tea.Model, tea.Cmd) {
	a.State.Status = StatusPrompting
	a.State.FocusedPane = PaneInput

	isPassword, _ := msg["is_password"].(bool)
	a.State.IsPasswordPrompt = isPassword

	if isPassword {
		a.input.EchoMode = textinput.EchoPassword
	} else {
		a.input.EchoMode = textinput.EchoNormal
	}

	return a, tea.Batch(a.input.Focus(), WaitForDaemonMsg(a.DaemonReader))
}

func (a *App) handleDaemonConnected(msg map[string]any) (tea.Model, tea.Cmd) {
	a.State.Status = StatusConnected
	a.State.ReconnectAttempts = 0
	a.State.ReconnectConnID = ""

	if ip, ok := msg["ip"].(string); ok && ip != "" {
		a.State.IP = ip
	}
	if pid, ok := msg["pid"].(float64); ok && pid != 0 {
		a.State.PID = int(pid)
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonDisconnectedEvent() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusQuitting {
		return a, WaitForDaemonMsg(a.DaemonReader)
	}

	wasConnected := a.State.Status == StatusConnected

	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	a.State.ReconnectAttempts = 0
	a.State.ReconnectConnID = ""
	a.State.OutputLines = append(a.State.OutputLines, "--- Disconnected ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if wasConnected && a.State.Config.Settings.Reconnect && a.State.ActiveConnID != "" {
		a.State.Status = StatusReconnecting
		a.State.ReconnectConnID = a.State.ActiveConnID
		return a.attemptReconnect()
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonError(msg map[string]any) (tea.Model, tea.Cmd) {
	code, _ := msg["code"].(string)
	message, _ := msg["message"].(string)

	a.State.OutputLines = append(a.State.OutputLines,
		fmt.Sprintf("\x1b[31mError [%s]: %s\x1b[0m", code, message))
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
		return a, tea.Batch(scheduleReconnectTick(), WaitForDaemonMsg(a.DaemonReader))
	}

	a.State.Status = StatusDisconnected
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleKicked() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[33mAnother client connected. Exiting...\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	return a, tea.Quit
}

func (a *App) handleDaemonCleanupStep(msg map[string]any) (tea.Model, tea.Cmd) {
	if line, ok := msg["line"].(string); ok {
		a.State.OutputLines = append(a.State.OutputLines, line)
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonCleanupDone() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines, "--- Cleanup complete ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	if a.State.Status == StatusQuitting {
		return a, tea.Quit
	}
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonDisconnected() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[31mLost connection to daemon. Exiting...\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	return a, tea.Quit
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

func (a *App) handleReconnectTick() (tea.Model, tea.Cmd) {
	if a.State.Status != StatusReconnecting || a.State.ReconnectConnID == "" {
		return a, nil
	}
	return a.attemptReconnect()
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

func (a *App) handleDetach() (tea.Model, tea.Cmd) {
	if a.DaemonConn != nil {
		a.DaemonConn.Close()
	}
	return a, tea.Quit
}

func (a *App) handleQuit() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusConnected || a.State.Status == StatusConnecting {
		a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})
	}
	if a.DaemonConn != nil {
		a.DaemonConn.Close()
	}
	return a, tea.Quit
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

	case key.Matches(msg, a.Keys.Detach):
		return a.handleDetach()

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

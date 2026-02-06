package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
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
			if key.Matches(keyMsg, a.Keys.Detach) && a.State.FormKind == FormUpdateNotice {
				a.State.ActiveForm = nil
				a.State.FormKind = FormNone
				a.State.FormData = nil
				return a, nil
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

	case resetTimeoutMsg:
		return a.handleResetTimeout()

	case restartTimeoutMsg:
		return a.handleRestartTimeout()

	case daemonRestartedMsg:
		return a.handleDaemonRestarted(msg)

	case daemonRestartFailedMsg:
		return a.handleDaemonRestartFailed(msg)

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
	case "log_range":
		return a.handleLogRange(msg.Raw)
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
	case "reconnecting":
		return a.handleDaemonReconnecting(msg.Raw)
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleHelloResponse(msg map[string]any) (tea.Model, tea.Cmd) {
	compatible, _ := msg["compatible"].(bool)
	if !compatible {
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogError("Daemon version mismatch. Please restart the daemon."))
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
	if totalLines, ok := msg["total_log_lines"].(float64); ok {
		a.State.TotalLogLines = int(totalLines)
		if a.State.TotalLogLines > 0 {
			from := max(0, a.State.TotalLogLines-MaxLoadedLines)
			a.requestLogs(from, a.State.TotalLogLines)
		}
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonLog(msg map[string]any) (tea.Model, tea.Cmd) {
	line, ok := msg["line"].(string)
	if !ok {
		return a, WaitForDaemonMsg(a.DaemonReader)
	}

	lineNum := 0
	if ln, ok := msg["line_number"].(float64); ok {
		lineNum = int(ln)
	}

	a.State.TotalLogLines = lineNum + 1

	if lineNum == a.State.LogLoadedTo {
		a.State.OutputLines = append(a.State.OutputLines, line)
		a.State.LogLoadedTo++

		if len(a.State.OutputLines) > MaxLoadedLines {
			excess := len(a.State.OutputLines) - MaxLoadedLines
			a.State.OutputLines = a.State.OutputLines[excess:]
			a.State.LogLoadedFrom += excess
		}

		a.viewport.SetContent(a.renderOutput())
		if a.viewport.AtBottom() || a.State.FocusedPane != PaneOutput {
			a.viewport.GotoBottom()
		}
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

	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	a.State.ReconnectAttempts = 0
	a.State.ReconnectConnID = ""
	a.State.OutputLines = append(a.State.OutputLines, "--- Disconnected ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonError(msg map[string]any) (tea.Model, tea.Cmd) {
	code, _ := msg["code"].(string)
	message, _ := msg["message"].(string)

	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogError(fmt.Sprintf("Error [%s]: %s", code, message)))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleKicked() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogWarning("Another client connected. Exiting..."))
	a.viewport.SetContent(a.renderOutput())
	return a, tea.Quit
}

func (a *App) handleDaemonDisconnected() (tea.Model, tea.Cmd) {
	if a.State.RestartingDaemon {
		return a, nil
	}
	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogError("Lost connection to daemon. Exiting..."))
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
			ui.LogWarning("[Reset cancelled - timeout]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, nil
}

func (a *App) handleRestartTimeout() (tea.Model, tea.Cmd) {
	if a.State.RestartPending {
		a.State.RestartPending = false
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogWarning("[Restart cancelled - timeout]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}
	return a, nil
}

func (a *App) handleRestartDaemon() (tea.Model, tea.Cmd) {
	if a.State.RestartingDaemon {
		return a, nil
	}
	if !a.State.RestartPending {
		a.State.RestartPending = true
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogWarning("[Press R again to restart daemon]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, scheduleRestartTimeout()
	}

	a.State.RestartPending = false
	a.State.RestartingDaemon = true
	a.State.ReconnectCountdown = 0
	a.State.ReconnectConnID = ""
	a.State.ReconnectAttempts = 0
	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogWarning("[Restarting daemon...]"))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()
	return a, a.restartDaemonCmd()
}

func (a *App) handleDaemonRestarted(msg daemonRestartedMsg) (tea.Model, tea.Cmd) {
	a.State.RestartingDaemon = false
	a.State.RestartPending = false
	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	a.State.ReconnectAttempts = 0
	a.State.ReconnectCountdown = 0
	a.State.ReconnectConnID = ""
	a.State.DisconnectRequested = false
	a.State.TotalLogLines = 0
	a.State.LogLoadedFrom = 0
	a.State.LogLoadedTo = 0
	a.State.OutputLines = []string{ui.LogWarning("[Daemon restarted]")}
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	a.DaemonConn = msg.Conn
	a.DaemonReader = msg.Reader

	a.SendToDaemon(daemon.ConfigUpdateCmd{
		Type:   "config_update",
		Config: *a.State.Config,
	})
	a.SendToDaemon(daemon.GetStateCmd{Type: "get_state"})
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) handleDaemonRestartFailed(msg daemonRestartFailedMsg) (tea.Model, tea.Cmd) {
	a.State.RestartingDaemon = false
	a.State.RestartPending = false
	a.State.Status = StatusDisconnected
	a.State.ActiveConnID = ""
	a.State.IP = ""
	a.State.PID = 0
	a.State.ReconnectAttempts = 0
	a.State.ReconnectCountdown = 0
	a.State.ReconnectConnID = ""
	a.State.DisconnectRequested = false
	a.DaemonConn = nil
	a.DaemonReader = nil
	errMsg := "unknown error"
	if msg.Err != nil {
		errMsg = msg.Err.Error()
	}
	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogError("[Daemon restart failed: "+errMsg+"]"))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()
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
	if a.State.ShowingHelp {
		return a.handleHelpKeys(msg)
	}

	if key.Matches(msg, a.Keys.Help) {
		a.State.ShowingHelp = true
		a.State.HelpScroll = 0
		return a, nil
	}

	if a.State.ReconnectCountdown > 0 && key.Matches(msg, a.Keys.Cancel) {
		a.State.ReconnectCountdown = 0
		a.State.ReconnectConnID = ""
		a.State.ReconnectAttempts = 0
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogWarning("[Reconnect cancelled]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	}

	if a.State.RestartPending && (!key.Matches(msg, a.Keys.RestartDaemon) || a.State.FocusedPane != PaneStatus) {
		a.State.RestartPending = false
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
		if key.Matches(msg, a.Keys.RestartDaemon) {
			return a.handleRestartDaemon()
		}
		if key.Matches(msg, a.Keys.Cleanup) {
			return a.cleanup()
		}
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

func (a *App) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.Keys.Cancel), key.Matches(msg, a.Keys.Help), key.Matches(msg, a.Keys.Detach):
		a.State.ShowingHelp = false
		a.State.HelpScroll = 0
		return a, nil
	case key.Matches(msg, a.Keys.Quit):
		return a.handleQuit()
	case key.Matches(msg, a.Keys.ScrollUp):
		if a.State.HelpScroll > 0 {
			a.State.HelpScroll--
		}
		return a, nil
	case key.Matches(msg, a.Keys.ScrollDown):
		a.State.HelpScroll++
		return a, nil
	case key.Matches(msg, a.Keys.ScrollToTop):
		a.State.HelpScroll = 0
		return a, nil
	case key.Matches(msg, a.Keys.ScrollToBottom):
		a.State.HelpScroll = 999
		return a, nil
	case key.Matches(msg, a.Keys.PageUp):
		a.State.HelpScroll = max(a.State.HelpScroll-5, 0)
		return a, nil
	case key.Matches(msg, a.Keys.PageDown):
		a.State.HelpScroll += 5
		return a, nil
	}
	return a, nil
}

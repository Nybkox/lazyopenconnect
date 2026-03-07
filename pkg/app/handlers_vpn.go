package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

func (a *App) connect() (tea.Model, tea.Cmd) {
	conn := a.State.SelectedConnection()
	if conn == nil {
		return a, nil
	}

	if a.State.Status != StatusDisconnected {
		return a, nil
	}

	var password string
	var passwordWarning string
	if conn.HasPassword {
		storedPassword, err := helpers.GetPassword(conn.ID)
		if err != nil {
			passwordWarning = ui.LogWarning("Failed to read saved password; daemon will prompt if needed.")
		} else {
			password = storedPassword
		}
	}

	a.State.Status = StatusConnecting
	a.State.ActiveConnID = conn.ID
	a.State.OutputLines = []string{}
	a.State.TotalLogLines = 0
	a.State.LogLoadedFrom = 0
	a.State.LogLoadedTo = 0
	if passwordWarning != "" {
		a.State.OutputLines = append(a.State.OutputLines, passwordWarning)
	}
	a.viewport.SetContent(a.renderOutput())

	a.SendToDaemon(daemon.ConnectCmd{
		Type:     "connect",
		ConnID:   conn.ID,
		Password: password,
	})

	return a, tea.Batch(spinnerTick(), scheduleConnectionTimeout())
}

func (a *App) handleConnectionTimeout() (tea.Model, tea.Cmd) {
	if a.State.Status != StatusConnecting {
		return a, nil
	}

	a.State.OutputLines = append(a.State.OutputLines,
		ui.LogError("Connection timed out after 30s"))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})

	return a, nil
}

func (a *App) disconnect() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusReconnecting {
		a.State.Status = StatusDisconnected
		a.State.ReconnectAttempts = 0
		a.State.ReconnectConnID = ""
		a.State.ActiveConnID = ""
		a.State.OutputLines = append(a.State.OutputLines, "--- Reconnect cancelled ---")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})
		return a, nil
	}

	if a.State.Status == StatusDisconnected {
		return a, nil
	}

	a.State.ReconnectConnID = ""

	a.State.OutputLines = append(a.State.OutputLines, "--- Disconnecting ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})

	return a, nil
}

func (a *App) cleanup() (tea.Model, tea.Cmd) {
	a.SendToDaemon(daemon.CleanupCmd{Type: "cleanup"})
	return a, nil
}

func (a *App) renderOutput() string {
	var output string
	for _, line := range a.State.OutputLines {
		output += line + "\n"
	}
	return output
}

func (a *App) handleDaemonReconnecting(msg daemon.ReconnectingMsg) (tea.Model, tea.Cmd) {
	a.State.Status = StatusReconnecting
	a.State.ActiveConnID = msg.ConnID
	a.State.ReconnectAttempts = msg.Attempt

	if msg.Attempt == 0 {
		return a, tea.Batch(spinnerTick(), WaitForDaemonMsg(a.DaemonReader))
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

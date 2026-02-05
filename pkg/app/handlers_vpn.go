package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
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
	if conn.HasPassword {
		password, _ = helpers.GetPassword(conn.ID)
	}

	a.State.Status = StatusConnecting
	a.State.ActiveConnID = conn.ID
	a.State.OutputLines = []string{}
	a.State.TotalLogLines = 0
	a.State.LogLoadedFrom = 0
	a.State.LogLoadedTo = 0
	a.viewport.SetContent(a.renderOutput())

	a.SendToDaemon(daemon.ConnectCmd{
		Type:     "connect",
		ConnID:   conn.ID,
		Password: password,
	})

	return a, spinnerTick()
}

func (a *App) disconnect() (tea.Model, tea.Cmd) {
	if a.State.Status == StatusReconnecting {
		a.State.Status = StatusDisconnected
		a.State.ReconnectAttempts = 0
		a.State.ReconnectConnID = ""
		a.State.ActiveConnID = ""
		a.State.DisconnectRequested = true
		a.State.OutputLines = append(a.State.OutputLines, "--- Reconnect cancelled ---")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})
		return a, nil
	}

	if a.State.Status == StatusDisconnected {
		return a, nil
	}

	a.State.DisconnectRequested = true
	a.State.ReconnectConnID = ""

	a.State.OutputLines = append(a.State.OutputLines, "--- Disconnecting ---")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	a.SendToDaemon(daemon.DisconnectCmd{Type: "disconnect"})

	return a, nil
}

func (a *App) cleanup() (tea.Model, tea.Cmd) {
	a.State.OutputLines = append(a.State.OutputLines, "--- Running cleanup ---")
	a.viewport.SetContent(a.renderOutput())
	return a, tea.Batch(
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) renderOutput() string {
	var output string
	for _, line := range a.State.OutputLines {
		output += line + "\n"
	}
	return output
}

func (a *App) handleDaemonReconnecting(msg map[string]any) (tea.Model, tea.Cmd) {
	connID, _ := msg["conn_id"].(string)
	attempt, _ := msg["attempt"].(float64)

	a.State.Status = StatusReconnecting
	a.State.ActiveConnID = connID
	a.State.ReconnectAttempts = int(attempt)

	if attempt == 0 {
		return a, tea.Batch(spinnerTick(), WaitForDaemonMsg(a.DaemonReader))
	}

	return a, WaitForDaemonMsg(a.DaemonReader)
}

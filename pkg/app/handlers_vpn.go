package app

import (
	"fmt"

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
		a.State.OutputLines = append(a.State.OutputLines, "--- Reconnect cancelled ---")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
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

func (a *App) attemptReconnect() (tea.Model, tea.Cmd) {
	a.State.ReconnectAttempts++

	conn := a.State.FindConnectionByID(a.State.ReconnectConnID)
	if conn == nil {
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[31mReconnect failed: connection not found\x1b[0m")
		return a.reconnectFailed()
	}

	a.State.OutputLines = append(a.State.OutputLines,
		fmt.Sprintf("\x1b[33mReconnecting... (attempt %d/%d)\x1b[0m",
			a.State.ReconnectAttempts, maxReconnectAttempts))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	var password string
	if conn.HasPassword {
		password, _ = helpers.GetPassword(conn.ID)
	}

	a.State.Status = StatusConnecting

	a.SendToDaemon(daemon.ConnectCmd{
		Type:     "connect",
		ConnID:   conn.ID,
		Password: password,
	})

	return a, spinnerTick()
}

func (a *App) reconnectFailed() (tea.Model, tea.Cmd) {
	a.State.Status = StatusQuitting
	a.State.ActiveConnID = ""
	a.State.ReconnectConnID = ""
	a.State.ReconnectAttempts = 0
	a.State.PID = 0
	a.State.IP = ""

	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[31mAll reconnect attempts failed. Exiting...\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	return a, tea.Batch(
		helpers.RunCleanup(&a.State.Config.Settings),
		helpers.WaitForCleanupStep(),
	)
}

func (a *App) startReconnect() (tea.Model, tea.Cmd) {
	conn := a.State.FindConnectionByID(a.State.ReconnectConnID)
	a.State.ReconnectConnID = ""
	a.State.ReconnectCountdown = 0

	if conn == nil {
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[31m[Reconnect failed: connection not found]\x1b[0m")
		a.State.ReconnectAttempts = 0
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	}

	var password string
	if conn.HasPassword {
		password, _ = helpers.GetPassword(conn.ID)
	}

	a.State.Status = StatusConnecting
	a.State.ActiveConnID = conn.ID

	a.State.OutputLines = append(a.State.OutputLines,
		fmt.Sprintf("\x1b[33m[Reconnecting to %s...]\x1b[0m", conn.Name))
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	a.SendToDaemon(daemon.ConnectCmd{
		Type:     "connect",
		ConnID:   conn.ID,
		Password: password,
	})

	return a, spinnerTick()
}

func (a *App) renderOutput() string {
	var output string
	for _, line := range a.State.OutputLines {
		output += line + "\n"
	}
	return output
}

package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type spinnerTickMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

type connectionTimeoutMsg struct {
	at time.Time
}

const connectionTimeout = 10 * time.Second

func (a *App) scheduleConnectionTimeout() tea.Cmd {
	at := a.State.LastOutputTime
	return tea.Tick(connectionTimeout, func(time.Time) tea.Msg {
		return connectionTimeoutMsg{at: at}
	})
}

type reconnectTickMsg struct{}

const (
	reconnectDelay       = 5 * time.Second
	maxReconnectAttempts = 3
)

func scheduleReconnectTick() tea.Cmd {
	return tea.Tick(reconnectDelay, func(time.Time) tea.Msg {
		return reconnectTickMsg{}
	})
}

type resetTimeoutMsg struct{}

const resetConfirmTimeout = 2 * time.Second

func scheduleResetTimeout() tea.Cmd {
	return tea.Tick(resetConfirmTimeout, func(time.Time) tea.Msg {
		return resetTimeoutMsg{}
	})
}

type externalCheckTickMsg struct{}

func (a *App) scheduleExternalCheck() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return externalCheckTickMsg{}
	})
}

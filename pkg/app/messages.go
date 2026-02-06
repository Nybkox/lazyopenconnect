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

type connectionTimeoutMsg struct{}

const connectionTimeout = 30 * time.Second

func scheduleConnectionTimeout() tea.Cmd {
	return tea.Tick(connectionTimeout, func(time.Time) tea.Msg {
		return connectionTimeoutMsg{}
	})
}

type resetTimeoutMsg struct{}

const resetConfirmTimeout = 2 * time.Second

func scheduleResetTimeout() tea.Cmd {
	return tea.Tick(resetConfirmTimeout, func(time.Time) tea.Msg {
		return resetTimeoutMsg{}
	})
}

type restartTimeoutMsg struct{}

const restartConfirmTimeout = 2 * time.Second

func scheduleRestartTimeout() tea.Cmd {
	return tea.Tick(restartConfirmTimeout, func(time.Time) tea.Msg {
		return restartTimeoutMsg{}
	})
}

type clearLogsTimeoutMsg struct{}

const clearLogsConfirmTimeout = 2 * time.Second

func scheduleClearLogsTimeout() tea.Cmd {
	return tea.Tick(clearLogsConfirmTimeout, func(time.Time) tea.Msg {
		return clearLogsTimeoutMsg{}
	})
}

type externalCheckTickMsg struct{}

func (a *App) scheduleExternalCheck() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return externalCheckTickMsg{}
	})
}

type UpdateCheckMsg struct {
	Available bool
	Version   string
	Error     error
}

type UpdatePerformedMsg struct {
	Error error
}

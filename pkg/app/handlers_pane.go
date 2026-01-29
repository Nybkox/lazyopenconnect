package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) cycleFocus() {
	switch a.State.FocusedPane {
	case PaneStatus:
		a.State.FocusedPane = PaneConnections
	case PaneConnections:
		a.State.FocusedPane = PaneSettings
	case PaneSettings:
		a.State.FocusedPane = PaneOutput
	case PaneOutput:
		a.State.FocusedPane = PaneInput
	case PaneInput:
		a.State.FocusedPane = PaneStatus
	}
}

func (a *App) updateConnections(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	connCount := len(a.State.Config.Connections)
	visible := max(a.State.ConnectionsVisible, 1)

	switch {
	case key.Matches(msg, a.Keys.Up):
		if a.State.Selected > 0 {
			a.State.Selected--
			a.ensureConnectionVisible()
		}
	case key.Matches(msg, a.Keys.Down):
		if a.State.Selected < connCount-1 {
			a.State.Selected++
			a.ensureConnectionVisible()
		}
	case key.Matches(msg, a.Keys.PageUp):
		a.State.Selected = max(a.State.Selected-visible/2, 0)
		a.ensureConnectionVisible()
	case key.Matches(msg, a.Keys.PageDown):
		a.State.Selected = max(min(a.State.Selected+visible/2, connCount-1), 0)
		a.ensureConnectionVisible()
	case key.Matches(msg, a.Keys.ScrollToTop):
		a.State.Selected = 0
		a.State.ConnectionsScroll = 0
	case key.Matches(msg, a.Keys.ScrollToBottom):
		if connCount > 0 {
			a.State.Selected = connCount - 1
			a.ensureConnectionVisible()
		}
	case key.Matches(msg, a.Keys.Connect):
		return a.connect()
	case key.Matches(msg, a.Keys.Disconnect):
		return a.disconnect()
	case key.Matches(msg, a.Keys.Cleanup):
		return a.cleanup()
	case key.Matches(msg, a.Keys.New):
		return a.showNewConnForm()
	case key.Matches(msg, a.Keys.Edit):
		return a.showEditConnForm()
	case key.Matches(msg, a.Keys.Delete):
		return a.showDeleteConfirm()
	}
	return a, nil
}

func (a *App) ensureConnectionVisible() {
	visible := max(a.State.ConnectionsVisible, 1)
	connCount := len(a.State.Config.Connections)

	if a.State.Selected < a.State.ConnectionsScroll {
		a.State.ConnectionsScroll = a.State.Selected
	}
	if a.State.Selected >= a.State.ConnectionsScroll+visible {
		a.State.ConnectionsScroll = a.State.Selected - visible + 1
	}
	maxScroll := max(connCount-visible, 0)
	a.State.ConnectionsScroll = max(min(a.State.ConnectionsScroll, maxScroll), 0)
}

func (a *App) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.Keys.Edit), key.Matches(msg, a.Keys.Connect):
		a.State.ResetPending = false
		return a.showSettingsForm()

	case key.Matches(msg, a.Keys.Reset):
		if a.State.ResetPending {
			return a.resetSettings()
		}
		a.State.ResetPending = true
		a.State.OutputLines = append(a.State.OutputLines,
			"\x1b[33m[Press r again to reset settings to defaults]\x1b[0m")
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, scheduleResetTimeout()
	}

	if a.State.ResetPending {
		a.State.ResetPending = false
	}
	return a, nil
}

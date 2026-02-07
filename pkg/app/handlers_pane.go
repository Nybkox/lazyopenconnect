package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
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
	if a.State.FilterActive {
		return a.updateConnectionsFilter(msg)
	}

	connCount := a.State.FilteredConnectionCount()
	visible := max(a.State.ConnectionsVisible, 1)

	switch {
	case key.Matches(msg, a.Keys.Search):
		a.State.FilterActive = true
		a.State.FilterText = ""
		a.State.FilterIndices = nil
		a.State.Selected = 0
		a.State.ConnectionsScroll = 0
		return a, nil
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
	case key.Matches(msg, a.Keys.MoveUp):
		return a.moveConnectionUp()
	case key.Matches(msg, a.Keys.MoveDown):
		return a.moveConnectionDown()
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

func (a *App) updateConnectionsFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.Keys.Cancel):
		a.State.FilterActive = false
		a.State.FilterText = ""
		a.State.FilterIndices = nil
		a.State.Selected = 0
		a.State.ConnectionsScroll = 0
		return a, nil
	case key.Matches(msg, a.Keys.Connect):
		if a.State.FilteredConnectionCount() > 0 {
			realIdx := a.State.RealIndex(a.State.Selected)
			a.State.FilterActive = false
			a.State.FilterText = ""
			a.State.FilterIndices = nil
			a.State.Selected = realIdx
			return a.connect()
		}
		return a, nil
	case key.Matches(msg, a.Keys.Up):
		if a.State.Selected > 0 {
			a.State.Selected--
			a.ensureConnectionVisible()
		}
	case key.Matches(msg, a.Keys.Down):
		count := a.State.FilteredConnectionCount()
		if a.State.Selected < count-1 {
			a.State.Selected++
			a.ensureConnectionVisible()
		}
	default:
		str := msg.String()
		if str == "backspace" {
			if len(a.State.FilterText) > 0 {
				a.State.FilterText = a.State.FilterText[:len(a.State.FilterText)-1]
				a.State.UpdateFilter()
			}
		} else if len(str) == 1 && str[0] >= 32 && str[0] < 127 {
			a.State.FilterText += str
			a.State.UpdateFilter()
		}
	}
	return a, nil
}

func (a *App) moveConnectionUp() (tea.Model, tea.Cmd) {
	if a.State.FilterActive || a.State.Selected <= 0 {
		return a, nil
	}
	i := a.State.Selected
	conns := a.State.Config.Connections
	conns[i-1], conns[i] = conns[i], conns[i-1]
	a.State.Selected--
	a.ensureConnectionVisible()
	_ = helpers.SaveConfig(a.State.Config)
	a.syncConfigToDaemon()
	return a, nil
}

func (a *App) moveConnectionDown() (tea.Model, tea.Cmd) {
	if a.State.FilterActive {
		return a, nil
	}
	connCount := len(a.State.Config.Connections)
	if a.State.Selected >= connCount-1 {
		return a, nil
	}
	i := a.State.Selected
	conns := a.State.Config.Connections
	conns[i], conns[i+1] = conns[i+1], conns[i]
	a.State.Selected++
	a.ensureConnectionVisible()
	_ = helpers.SaveConfig(a.State.Config)
	a.syncConfigToDaemon()
	return a, nil
}

func (a *App) ensureConnectionVisible() {
	visible := max(a.State.ConnectionsVisible, 1)
	connCount := a.State.FilteredConnectionCount()

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
			ui.LogWarning("[Press r again to reset settings to defaults]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, scheduleResetTimeout()
	}

	if a.State.ResetPending {
		a.State.ResetPending = false
	}
	return a, nil
}

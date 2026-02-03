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

func (a *App) updateOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.Keys.Export):
		return a.showExportLogsForm()
	case key.Matches(msg, a.Keys.CopyLogs):
		if err := helpers.CopyVpnLogToClipboard(); err != nil {
			a.State.OutputLines = append(a.State.OutputLines, ui.LogError("[Copy failed: "+err.Error()+"]"))
		} else {
			a.State.OutputLines = append(a.State.OutputLines, ui.LogSuccess("[Logs copied to clipboard]"))
		}
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	case key.Matches(msg, a.Keys.ScrollUp):
		a.viewport.ScrollUp(1)
	case key.Matches(msg, a.Keys.ScrollDown):
		a.viewport.ScrollDown(1)
	case key.Matches(msg, a.Keys.PageUp):
		a.viewport.HalfPageUp()
	case key.Matches(msg, a.Keys.PageDown):
		a.viewport.HalfPageDown()
	case key.Matches(msg, a.Keys.ScrollToTop):
		a.viewport.GotoTop()
	case key.Matches(msg, a.Keys.ScrollToBottom):
		a.viewport.GotoBottom()
	}
	a.maybeFetchLogs()
	return a, nil
}

func (a *App) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.Keys.Quit) {
		return a.handleQuit()
	}

	if msg.String() == "ctrl+d" {
		if a.State.Status == StatusExternal || a.State.Status == StatusConnected || a.State.Status == StatusReconnecting {
			return a.disconnect()
		}
		return a, nil
	}

	if key.Matches(msg, a.Keys.Submit) {
		if a.DaemonConn != nil && a.input.Value() != "" {
			value := a.input.Value()

			a.SendToDaemon(daemon.InputCmd{
				Type:  "input",
				Value: value,
			})

			displayValue := value
			if a.State.IsPasswordPrompt {
				displayValue = fmt.Sprintf("**** (%d chars)", len(value))
			}
			a.State.OutputLines = append(a.State.OutputLines, "> "+displayValue)

			a.input.SetValue("")
			a.input.EchoMode = textinput.EchoNormal
			a.State.IsPasswordPrompt = false
			a.State.Status = StatusConnecting

			a.viewport.SetContent(a.renderOutput())
			a.viewport.GotoBottom()
		}
		return a, nil
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

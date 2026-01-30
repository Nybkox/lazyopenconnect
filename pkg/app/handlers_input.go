package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
)

func (a *App) updateOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.Keys.Export):
		return a.showExportLogsForm()
	case key.Matches(msg, a.Keys.CopyLogs):
		if err := helpers.CopyLogsToClipboard(a.State.OutputLines); err != nil {
			a.State.OutputLines = append(a.State.OutputLines,
				"\x1b[31m[Copy failed: "+err.Error()+"]\x1b[0m")
		} else {
			a.State.OutputLines = append(a.State.OutputLines,
				"\x1b[32m[Logs copied to clipboard]\x1b[0m")
		}
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	case key.Matches(msg, a.Keys.ScrollUp):
		a.viewport.LineUp(1)
	case key.Matches(msg, a.Keys.ScrollDown):
		a.viewport.LineDown(1)
	case key.Matches(msg, a.Keys.PageUp):
		a.viewport.HalfViewUp()
	case key.Matches(msg, a.Keys.PageDown):
		a.viewport.HalfViewDown()
	case key.Matches(msg, a.Keys.ScrollToTop):
		a.viewport.GotoTop()
	case key.Matches(msg, a.Keys.ScrollToBottom):
		a.viewport.GotoBottom()
	}
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
		if a.State.Stdin != nil && a.input.Value() != "" {
			value := a.input.Value()
			a.State.Stdin.Write([]byte(value + "\n"))

			displayValue := value
			if a.State.IsPasswordPrompt {
				displayValue = fmt.Sprintf("**** (%d chars)", len(value))
			}
			a.State.OutputLines = append(a.State.OutputLines, "> "+displayValue)

			a.input.SetValue("")
			a.input.EchoMode = textinput.EchoNormal
			a.State.IsPasswordPrompt = false

			a.viewport.SetContent(a.renderOutput())
			a.viewport.GotoBottom()
		}
		return a, nil
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

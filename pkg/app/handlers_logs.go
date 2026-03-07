package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
)

const MaxLoadedLines = 1000

func (a *App) handleLogRange(msg daemon.LogRangeMsg) (tea.Model, tea.Cmd) {
	a.State.TotalLogLines = msg.TotalLines
	a.State.LogLoadedFrom = msg.From
	a.State.OutputLines = append([]string(nil), msg.Lines...)
	a.State.LogLoadedTo = msg.From + len(a.State.OutputLines)

	a.viewport.SetContent(a.renderOutput())
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) requestLogs(from, to int) {
	a.SendToDaemon(daemon.GetLogsCmd{Type: "get_logs", From: from, To: to})
}

func (a *App) shouldFetchLogs() (bool, int, int) {
	if a.State.TotalLogLines == 0 {
		return false, 0, 0
	}

	visibleStart := a.State.LogLoadedFrom + a.viewport.YOffset
	visibleEnd := visibleStart + a.viewport.Height

	if visibleStart < a.State.LogLoadedFrom || visibleEnd > a.State.LogLoadedTo {
		center := (visibleStart + visibleEnd) / 2
		from := center - MaxLoadedLines/2
		to := from + MaxLoadedLines

		if from < 0 {
			from = 0
			to = min(MaxLoadedLines, a.State.TotalLogLines)
		}
		if to > a.State.TotalLogLines {
			to = a.State.TotalLogLines
			from = max(0, to-MaxLoadedLines)
		}

		return true, from, to
	}

	return false, 0, 0
}

func (a *App) maybeFetchLogs() {
	if needFetch, from, to := a.shouldFetchLogs(); needFetch {
		a.requestLogs(from, to)
	}
}

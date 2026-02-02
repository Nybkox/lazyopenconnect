package app

import tea "github.com/charmbracelet/bubbletea"

const MaxLoadedLines = 1000

func (a *App) handleLogRange(msg map[string]any) (tea.Model, tea.Cmd) {
	from := 0
	if f, ok := msg["from"].(float64); ok {
		from = int(f)
	}

	totalLines := 0
	if t, ok := msg["total_lines"].(float64); ok {
		totalLines = int(t)
	}

	lines, ok := msg["lines"].([]any)
	if !ok {
		return a, WaitForDaemonMsg(a.DaemonReader)
	}

	a.State.TotalLogLines = totalLines
	a.State.LogLoadedFrom = from
	a.State.OutputLines = make([]string, 0, len(lines))
	for _, l := range lines {
		if s, ok := l.(string); ok {
			a.State.OutputLines = append(a.State.OutputLines, s)
		}
	}
	a.State.LogLoadedTo = from + len(a.State.OutputLines)

	a.viewport.SetContent(a.renderOutput())
	return a, WaitForDaemonMsg(a.DaemonReader)
}

func (a *App) requestLogs(from, to int) {
	a.SendToDaemon(map[string]any{
		"type": "get_logs",
		"from": from,
		"to":   to,
	})
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

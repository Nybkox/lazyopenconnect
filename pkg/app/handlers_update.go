package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

func (a *App) handleUpdateCheck(msg UpdateCheckMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil || !msg.Available {
		return a, nil
	}

	a.State.UpdateAvailable = true
	a.State.UpdateVersion = msg.Version

	if a.State.Config.Settings.SkipVersionUpdate != msg.Version {
		data := &UpdateNoticeData{Action: "later"}
		a.State.FormKind = FormUpdateNotice
		a.State.FormData = data
		a.State.ActiveForm = NewUpdateNoticeForm(msg.Version, data, helpers.IsHomebrewInstall())
		return a, a.State.ActiveForm.Init()
	}

	return a, nil
}

func (a *App) handleUpdateFormComplete() (tea.Model, tea.Cmd) {
	if a.State.ActiveForm == nil || a.State.FormData == nil {
		a.State.ActiveForm = nil
		a.State.FormKind = FormNone
		a.State.FormData = nil
		return a, nil
	}

	data := a.State.FormData.(*UpdateNoticeData)

	switch data.Action {
	case "update":
		a.State.ActiveForm = nil
		a.State.FormKind = FormNone
		a.State.FormData = nil
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogWarning("[Downloading update...]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, performUpdateCmd()

	case "skip":
		a.State.Config.Settings.SkipVersionUpdate = a.State.UpdateVersion
		_ = helpers.SaveConfig(a.State.Config)
	}

	a.State.ActiveForm = nil
	a.State.FormKind = FormNone
	a.State.FormData = nil

	return a, nil
}

func (a *App) handleUpdatePerformed(msg UpdatePerformedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.State.OutputLines = append(a.State.OutputLines,
			ui.LogError("[Update failed: "+msg.Error.Error()+"]"))
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
		return a, nil
	}

	fmt.Println("\n" + ui.LogSuccess("Update successful! Please restart lazyopenconnect."))
	return a, tea.Quit
}

func CheckForUpdates() tea.Cmd {
	return func() tea.Msg {
		info, err := helpers.CheckForUpdate()
		if err != nil {
			return UpdateCheckMsg{Error: err}
		}
		return UpdateCheckMsg{
			Available: info.Available,
			Version:   info.Latest,
		}
	}
}

func performUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		err := helpers.PerformUpdate()
		return UpdatePerformedMsg{Error: err}
	}
}

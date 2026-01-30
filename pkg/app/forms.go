package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/google/uuid"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func (a *App) formWidth() int {
	return max(min(a.State.Width/2, 60), 40)
}

func (a *App) showNewConnForm() (tea.Model, tea.Cmd) {
	data := helpers.NewConnectionFormData(nil)
	form := helpers.NewConnectionForm(data, a.formWidth(), false)

	a.State.ActiveForm = form
	a.State.FormKind = FormNewConn
	a.State.FormData = data

	return a, form.Init()
}

func (a *App) showEditConnForm() (tea.Model, tea.Cmd) {
	conn := a.State.SelectedConnection()
	if conn == nil {
		return a, nil
	}

	data := helpers.NewConnectionFormData(conn)
	form := helpers.NewConnectionForm(data, a.formWidth(), true)

	a.State.ActiveForm = form
	a.State.FormKind = FormEditConn
	a.State.FormData = data

	return a, form.Init()
}

func (a *App) showDeleteConfirm() (tea.Model, tea.Cmd) {
	conn := a.State.SelectedConnection()
	if conn == nil {
		return a, nil
	}

	data := &helpers.DeleteConfirmData{}
	form := helpers.NewDeleteConfirmForm(conn.Name, data, a.formWidth())

	a.State.ActiveForm = form
	a.State.FormKind = FormDeleteConfirm
	a.State.FormData = data

	return a, form.Init()
}

func (a *App) showSettingsForm() (tea.Model, tea.Cmd) {
	data := helpers.NewSettingsFormData(&a.State.Config.Settings)
	form := helpers.NewSettingsForm(data, a.formWidth())

	a.State.ActiveForm = form
	a.State.FormKind = FormSettings
	a.State.FormData = data

	return a, form.Init()
}

func (a *App) showExportLogsForm() (tea.Model, tea.Cmd) {
	data := helpers.NewExportFormData()
	form := helpers.NewExportLogsForm(data, a.formWidth())

	a.State.ActiveForm = form
	a.State.FormKind = FormExportLogs
	a.State.FormData = data

	return a, form.Init()
}

func (a *App) resetSettings() (tea.Model, tea.Cmd) {
	a.State.ResetPending = false

	a.State.Config.Settings = models.Settings{
		DNS:         "",
		Reconnect:   false,
		AutoCleanup: true,
	}

	_ = helpers.SaveConfig(a.State.Config)

	a.State.OutputLines = append(a.State.OutputLines,
		"\x1b[32m[Settings reset to defaults]\x1b[0m")
	a.viewport.SetContent(a.renderOutput())
	a.viewport.GotoBottom()

	return a, nil
}

func (a *App) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := a.State.ActiveForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		a.State.ActiveForm = f
	}

	if a.State.ActiveForm.State == huh.StateCompleted {
		return a.handleFormComplete()
	}

	return a, cmd
}

func (a *App) handleFormComplete() (tea.Model, tea.Cmd) {
	defer func() {
		a.State.ActiveForm = nil
		a.State.FormKind = FormNone
		a.State.FormData = nil
	}()

	switch a.State.FormKind {
	case FormNewConn:
		data := a.State.FormData.(*helpers.ConnectionFormData)
		conn := data.ToConnection(nil)
		conn.ID = uuid.New().String()

		a.State.Config.Connections = append(a.State.Config.Connections, *conn)

		if strings.TrimSpace(data.Password) != "" {
			_ = helpers.SetPassword(conn.ID, data.Password)
		}

		_ = helpers.SaveConfig(a.State.Config)

	case FormEditConn:
		data := a.State.FormData.(*helpers.ConnectionFormData)
		existing := a.State.SelectedConnection()
		if existing != nil {
			conn := data.ToConnection(existing)
			a.State.Config.Connections[a.State.Selected] = *conn

			if strings.TrimSpace(data.Password) != "" {
				_ = helpers.SetPassword(conn.ID, data.Password)
			}

			_ = helpers.SaveConfig(a.State.Config)
		}

	case FormDeleteConfirm:
		data := a.State.FormData.(*helpers.DeleteConfirmData)
		if data.Confirmed {
			conn := a.State.SelectedConnection()
			if conn != nil {
				_ = helpers.DeletePassword(conn.ID)

				a.State.Config.Connections = append(
					a.State.Config.Connections[:a.State.Selected],
					a.State.Config.Connections[a.State.Selected+1:]...,
				)

				if a.State.Selected >= len(a.State.Config.Connections) && a.State.Selected > 0 {
					a.State.Selected--
				}

				_ = helpers.SaveConfig(a.State.Config)
			}
		}

	case FormSettings:
		data := a.State.FormData.(*helpers.SettingsFormData)
		a.State.Config.Settings = *data.ToSettings()

		_ = helpers.SaveConfig(a.State.Config)

	case FormExportLogs:
		data := a.State.FormData.(*helpers.ExportFormData)
		if err := helpers.ExportLogs(data.Path, a.State.OutputLines); err != nil {
			a.State.OutputLines = append(a.State.OutputLines,
				"\x1b[31m[Export failed: "+err.Error()+"]\x1b[0m")
		} else {
			a.State.OutputLines = append(a.State.OutputLines,
				"\x1b[32m[Logs exported to "+data.Path+"]\x1b[0m")
		}
		a.viewport.SetContent(a.renderOutput())
		a.viewport.GotoBottom()
	}

	return a, nil
}

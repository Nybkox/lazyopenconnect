package app

import (
	"bufio"
	"net"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

// RenderFunc is set externally to avoid import cycle
type RenderFunc func(state *State, spinnerFrame int) string

type App struct {
	State        *State
	Keys         KeyMap
	RenderView   RenderFunc
	DaemonConn   net.Conn
	DaemonReader *bufio.Reader
	viewport     viewport.Model
	input        textinput.Model
	spinnerFrame int
}

func New(cfg *models.Config) *App {
	vp := viewport.New(0, 0)
	ti := textinput.New()

	return &App{
		State:    NewState(cfg),
		Keys:     DefaultKeyMap(),
		viewport: vp,
		input:    ti,
	}
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
		CheckForUpdates(),
	}

	if a.DaemonReader != nil {
		cmds = append(cmds, WaitForDaemonMsg(a.DaemonReader))
	}

	return tea.Batch(cmds...)
}

func (a *App) View() string {
	if a.RenderView == nil {
		return "Loading..."
	}
	a.State.InputView = a.input.View()
	a.State.OutputView = a.viewport.View()
	return a.RenderView(a.State, a.spinnerFrame)
}

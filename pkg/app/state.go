package app

import (
	"io"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

type ConnStatus int

const (
	StatusDisconnected ConnStatus = iota
	StatusConnecting
	StatusPrompting
	StatusConnected
	StatusExternal
	StatusReconnecting
	StatusQuitting
)

type FocusedPane int

const (
	PaneStatus FocusedPane = iota
	PaneConnections
	PaneSettings
	PaneOutput
	PaneInput
)

type FormKind int

const (
	FormNone FormKind = iota
	FormNewConn
	FormEditConn
	FormSettings
	FormDeleteConfirm
	FormExportLogs
)

type State struct {
	Config *models.Config

	Selected         int
	ActiveConnID     string
	Status           ConnStatus
	IP               string
	PID              int
	Stdin            io.WriteCloser
	LastOutputTime   time.Time
	IsPasswordPrompt bool
	ExternalHost     string

	DisconnectRequested bool
	ReconnectAttempts   int
	ReconnectCountdown  int
	ReconnectConnID     string

	FocusedPane        FocusedPane
	OutputLines        []string
	OutputScroll       int
	ConnectionsScroll  int
	ConnectionsVisible int
	Width              int
	Height             int
	InputView          string
	OutputView         string

	ActiveForm *huh.Form
	FormKind   FormKind
	FormData   any

	ResetPending bool
}

func NewState(cfg *models.Config) *State {
	return &State{
		Config:       cfg,
		Selected:     0,
		Status:       StatusDisconnected,
		FocusedPane:  PaneConnections,
		OutputLines:  []string{},
		OutputScroll: 0,
	}
}

func (s *State) ActiveConnection() *models.Connection {
	if s.ActiveConnID == "" {
		return nil
	}
	for i := range s.Config.Connections {
		if s.Config.Connections[i].ID == s.ActiveConnID {
			return &s.Config.Connections[i]
		}
	}
	return nil
}

func (s *State) SelectedConnection() *models.Connection {
	if len(s.Config.Connections) == 0 {
		return nil
	}
	if s.Selected < 0 || s.Selected >= len(s.Config.Connections) {
		return nil
	}
	return &s.Config.Connections[s.Selected]
}

func (s *State) MatchConnectionByHost(host string) *models.Connection {
	if host == "" {
		return nil
	}
	for i := range s.Config.Connections {
		if s.Config.Connections[i].Host == host {
			return &s.Config.Connections[i]
		}
	}
	return nil
}

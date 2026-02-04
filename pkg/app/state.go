package app

import (
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
	FormUpdateNotice
)

type State struct {
	Config *models.Config

	Selected         int
	ActiveConnID     string
	Status           ConnStatus
	IP               string
	PID              int
	IsPasswordPrompt bool
	ExternalHost     string

	DisconnectRequested bool
	ReconnectAttempts   int
	ReconnectCountdown  int
	ReconnectConnID     string

	FocusedPane        FocusedPane
	OutputLines        []string
	TotalLogLines      int
	LogLoadedFrom      int
	LogLoadedTo        int
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

	ResetPending     bool
	RestartPending   bool
	RestartingDaemon bool

	// Update notification
	UpdateAvailable bool
	UpdateVersion   string
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

func (s *State) FindConnectionByID(id string) *models.Connection {
	if id == "" {
		return nil
	}
	for i := range s.Config.Connections {
		if s.Config.Connections[i].ID == id {
			return &s.Config.Connections[i]
		}
	}
	return nil
}

func (s *State) ActiveConnection() *models.Connection {
	return s.FindConnectionByID(s.ActiveConnID)
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

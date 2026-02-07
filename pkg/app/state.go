package app

import (
	"strings"

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
	OutputYOffset      int
	OutputTotalLines   int
	OutputVisibleLines int
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
	ClearLogsPending bool
	RestartPending   bool
	RestartingDaemon bool

	// Update notification
	UpdateAvailable bool
	UpdateVersion   string

	ShowingHelp bool
	HelpScroll  int

	FilterActive  bool
	FilterText    string
	FilterIndices []int
}

func NewState(cfg *models.Config) *State {
	return &State{
		Config:      cfg,
		Selected:    0,
		Status:      StatusDisconnected,
		FocusedPane: PaneConnections,
		OutputLines: []string{},
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

func (s *State) RealIndex(selected int) int {
	if !s.FilterActive || len(s.FilterIndices) == 0 {
		return selected
	}
	if selected < 0 || selected >= len(s.FilterIndices) {
		return -1
	}
	return s.FilterIndices[selected]
}

func (s *State) SelectedConnection() *models.Connection {
	idx := s.RealIndex(s.Selected)
	if idx < 0 || idx >= len(s.Config.Connections) {
		return nil
	}
	return &s.Config.Connections[idx]
}

func (s *State) FilteredConnectionCount() int {
	if s.FilterActive {
		return len(s.FilterIndices)
	}
	return len(s.Config.Connections)
}

func (s *State) UpdateFilter() {
	if !s.FilterActive || s.FilterText == "" {
		s.FilterIndices = nil
		return
	}
	query := strings.ToLower(s.FilterText)
	s.FilterIndices = nil
	for i, conn := range s.Config.Connections {
		if strings.Contains(strings.ToLower(conn.Name), query) ||
			strings.Contains(strings.ToLower(conn.Host), query) {
			s.FilterIndices = append(s.FilterIndices, i)
		}
	}
	if s.Selected >= len(s.FilterIndices) {
		s.Selected = max(len(s.FilterIndices)-1, 0)
	}
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

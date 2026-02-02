package app

import (
	"bufio"
	"net"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
)

type DaemonMsg struct {
	Raw map[string]any
}

type DaemonDisconnectedMsg struct{}

func WaitForDaemonMsg(reader *bufio.Reader) tea.Cmd {
	return func() tea.Msg {
		msg, err := daemon.ReadMsg(reader)
		if err != nil {
			return DaemonDisconnectedMsg{}
		}
		return DaemonMsg{Raw: msg}
	}
}

func (a *App) SendToDaemon(msg any) {
	if a.DaemonConn != nil {
		daemon.WriteMsg(a.DaemonConn, msg)
	}
}

func (a *App) ConnectToDaemon(conn net.Conn) {
	a.DaemonConn = conn
	a.DaemonReader = bufio.NewReader(conn)
}

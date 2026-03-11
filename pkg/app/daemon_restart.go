package app

import (
	"bufio"
	"net"
	"os"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
	"github.com/Nybkox/lazyopenconnect/pkg/version"
)

type daemonRestartedMsg struct {
	Conn   net.Conn
	Reader *bufio.Reader
}

type daemonRestartFailedMsg struct {
	Err error
}

func (a *App) restartDaemonCmd() tea.Cmd {
	conn := a.DaemonConn
	return func() tea.Msg {
		socketPath, err := daemon.SocketPath()
		if err != nil {
			return daemonRestartFailedMsg{Err: err}
		}

		if conn != nil {
			_ = daemon.WriteMsg(conn, daemon.ShutdownCmd{Type: "shutdown"})
			_ = conn.Close()
		}

		_ = daemon.WaitForDaemonStop(socketPath, 2*time.Second)

		if err := daemon.SpawnDaemon(daemon.SpawnConfig{Debug: isDebugRun()}); err != nil {
			return daemonRestartFailedMsg{Err: err}
		}

		result, err := daemon.ConnectAndHello(socketPath, version.Current, 3*time.Second)
		if err != nil {
			return daemonRestartFailedMsg{Err: err}
		}
		return daemonRestartedMsg{Conn: result.Conn, Reader: result.Reader}
	}
}

func isDebugRun() bool {
	return slices.Contains(os.Args[1:], "--debug")
}

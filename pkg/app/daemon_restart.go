package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"slices"
	"syscall"
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
			conn.Close()
		}

		_ = waitForDaemonDown(socketPath, 2*time.Second)

		if err := spawnDaemon(isDebugRun()); err != nil {
			return daemonRestartFailedMsg{Err: err}
		}

		result, err := connectAndHello(socketPath, 3*time.Second)
		if err != nil {
			return daemonRestartFailedMsg{Err: err}
		}
		return daemonRestartedMsg(result)
	}
}

func isDebugRun() bool {
	return slices.Contains(os.Args[1:], "--debug")
}

func spawnDaemon(debug bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"daemon", "run"}
	if debug {
		args = append(args, "--debug")
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Process.Release()
}

func waitForDaemonDown(socketPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !daemonRunning(socketPath) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !daemonRunning(socketPath)
}

type helloResult struct {
	Conn   net.Conn
	Reader *bufio.Reader
}

func connectAndHello(socketPath string, timeout time.Duration) (helloResult, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		reader := bufio.NewReader(conn)
		if err := daemon.WriteMsg(conn, daemon.HelloCmd{Type: "hello", Version: version.Current}); err != nil {
			conn.Close()
			return helloResult{}, err
		}
		resp, err := daemon.ReadMsg(reader)
		if err != nil {
			conn.Close()
			return helloResult{}, err
		}
		data, err := json.Marshal(resp)
		if err != nil {
			conn.Close()
			return helloResult{}, err
		}
		var hello daemon.HelloResponse
		if err := json.Unmarshal(data, &hello); err != nil {
			conn.Close()
			return helloResult{}, err
		}
		if hello.Type != "hello_response" {
			conn.Close()
			return helloResult{}, errors.New("unexpected daemon response")
		}
		if !hello.Compatible {
			conn.Close()
			return helloResult{}, errors.New("daemon version mismatch")
		}
		return helloResult{Conn: conn, Reader: reader}, nil
	}
	return helloResult{}, errors.New("timeout connecting to daemon")
}

func daemonRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		os.Remove(socketPath)
		return false
	}
	conn.Close()
	return true
}

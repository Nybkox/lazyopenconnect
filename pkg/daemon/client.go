package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type DaemonConnStatus int

const (
	DaemonNotRunning DaemonConnStatus = iota
	DaemonReachable
	DaemonInaccessible
)

var ErrDaemonPrivilegeRequired = errors.New("daemon restart requires sudo")

type VersionMismatchError struct {
	ClientVersion string
	DaemonVersion string
}

func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("daemon version mismatch (client: %s, daemon: %s)", e.ClientVersion, e.DaemonVersion)
}

type HelloResult struct {
	Conn   net.Conn
	Reader *bufio.Reader
	Hello  HelloResponse
}

type SpawnConfig struct {
	Debug       bool
	Interactive bool
	Stdin       *os.File
	Stdout      *os.File
	Stderr      *os.File
}

func DaemonStatus(socketPath string) DaemonConnStatus {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		if isSocketPermissionErr(err) {
			if _, statErr := os.Stat(socketPath); statErr == nil {
				return DaemonInaccessible
			}
		}
		return DaemonNotRunning
	}
	_ = conn.Close()
	return DaemonReachable
}

func WaitForDaemonStart(socketPath string, timeout time.Duration) DaemonConnStatus {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := DaemonStatus(socketPath)
		if status != DaemonNotRunning {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	return DaemonNotRunning
}

func WaitForDaemonStop(socketPath string, timeout time.Duration) DaemonConnStatus {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := DaemonStatus(socketPath)
		if status == DaemonNotRunning {
			return status
		}
		time.Sleep(100 * time.Millisecond)
	}
	return DaemonStatus(socketPath)
}

func ConnectAndHello(socketPath, clientVersion string, timeout time.Duration) (HelloResult, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		reader := bufio.NewReader(conn)
		if err := WriteMsg(conn, HelloCmd{Type: "hello", Version: clientVersion}); err != nil {
			_ = conn.Close()
			return HelloResult{}, err
		}

		resp, err := ReadMsg(reader)
		if err != nil {
			_ = conn.Close()
			return HelloResult{}, err
		}

		var hello HelloResponse
		if err := resp.Decode(&hello); err != nil {
			_ = conn.Close()
			return HelloResult{}, err
		}
		if hello.Type != "hello_response" {
			_ = conn.Close()
			return HelloResult{}, errors.New("unexpected daemon response")
		}
		if !hello.Compatible {
			_ = conn.Close()
			return HelloResult{}, &VersionMismatchError{ClientVersion: clientVersion, DaemonVersion: hello.Version}
		}

		return HelloResult{Conn: conn, Reader: reader, Hello: hello}, nil
	}

	return HelloResult{}, errors.New("timeout connecting to daemon")
}

func SpawnDaemon(cfg SpawnConfig) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"daemon", "run"}
	if cfg.Debug {
		args = append(args, "--debug")
	}

	if os.Geteuid() == 0 {
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

	if !cfg.Interactive {
		return ErrDaemonPrivilegeRequired
	}

	fmt.Fprintln(cfg.Stdout, "Starting privileged daemon (sudo required)...")
	sudoArgs := append([]string{"-b", exe}, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	return cmd.Run()
}

func RequestShutdown(socketPath string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	return WriteMsg(conn, ShutdownCmd{Type: "shutdown"})
}

func isSocketPermissionErr(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}

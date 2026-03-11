package daemon

import (
	"bufio"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemonStatus(t *testing.T) {
	t.Run("not running", func(t *testing.T) {
		socketPath := tempSocketPath(t)
		if got := DaemonStatus(socketPath); got != DaemonNotRunning {
			t.Fatalf("DaemonStatus = %v, want %v", got, DaemonNotRunning)
		}
	})

	t.Run("reachable", func(t *testing.T) {
		socketPath := tempSocketPath(t)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("Listen returned error: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err == nil {
				_ = conn.Close()
			}
		}()

		if got := DaemonStatus(socketPath); got != DaemonReachable {
			t.Fatalf("DaemonStatus = %v, want %v", got, DaemonReachable)
		}
	})
}

func TestWaitForDaemonStart(t *testing.T) {
	socketPath := tempSocketPath(t)

	go func() {
		time.Sleep(100 * time.Millisecond)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			return
		}
		defer listener.Close()
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	if got := WaitForDaemonStart(socketPath, time.Second); got != DaemonReachable {
		t.Fatalf("WaitForDaemonStart = %v, want %v", got, DaemonReachable)
	}
}

func TestWaitForDaemonStop(t *testing.T) {
	socketPath := tempSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	if got := WaitForDaemonStop(socketPath, time.Second); got != DaemonNotRunning {
		t.Fatalf("WaitForDaemonStop = %v, want %v", got, DaemonNotRunning)
	}
}

func TestConnectAndHello(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		socketPath := tempSocketPath(t)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("Listen returned error: %v", err)
		}
		defer listener.Close()

		go serveHello(t, listener, HelloResponse{Type: "hello_response", Version: "1.2.3", Compatible: true})

		result, err := ConnectAndHello(socketPath, "1.2.3", time.Second)
		if err != nil {
			t.Fatalf("ConnectAndHello returned error: %v", err)
		}
		defer result.Conn.Close()

		if result.Hello.Version != "1.2.3" {
			t.Fatalf("Hello.Version = %q, want %q", result.Hello.Version, "1.2.3")
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		socketPath := tempSocketPath(t)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("Listen returned error: %v", err)
		}
		defer listener.Close()

		go serveHello(t, listener, HelloResponse{Type: "hello_response", Version: "9.9.9", Compatible: false})

		_, err = ConnectAndHello(socketPath, "1.2.3", time.Second)
		var mismatchErr *VersionMismatchError
		if !errors.As(err, &mismatchErr) {
			t.Fatalf("expected VersionMismatchError, got %v", err)
		}
		if mismatchErr.DaemonVersion != "9.9.9" {
			t.Fatalf("DaemonVersion = %q, want %q", mismatchErr.DaemonVersion, "9.9.9")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		socketPath := tempSocketPath(t)
		_, err := ConnectAndHello(socketPath, "1.2.3", 250*time.Millisecond)
		if err == nil || err.Error() != "timeout connecting to daemon" {
			t.Fatalf("expected timeout error, got %v", err)
		}
	})
}

func serveHello(t *testing.T, listener net.Listener, response HelloResponse) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := ReadMsg(bufio.NewReader(conn)); err != nil {
		t.Errorf("ReadMsg returned error: %v", err)
		return
	}
	if err := WriteMsg(conn, response); err != nil {
		t.Errorf("WriteMsg returned error: %v", err)
	}
}

func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "loc-")
	if err != nil {
		t.Fatalf("MkdirTemp returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return filepath.Join(dir, "d.sock")
}

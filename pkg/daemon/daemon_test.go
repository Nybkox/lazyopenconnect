package daemon

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func TestSanitizeConfig(t *testing.T) {
	t.Run("keeps valid connections and settings", func(t *testing.T) {
		cfg := models.Config{
			Connections: []models.Connection{
				{ID: "conn-1", Name: "Work VPN", Host: "vpn.work.com", Protocol: "anyconnect", Username: "alice", HasPassword: true},
			},
			Settings: models.Settings{
				DNS:             "8.8.8.8",
				Reconnect:       true,
				AutoCleanup:     true,
				TunnelInterface: "utun1",
				NetInterface:    "en1",
				WifiInterface:   "Wi-Fi",
			},
		}

		clean := sanitizeConfig(cfg)

		if len(clean.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(clean.Connections))
		}
		conn := clean.Connections[0]
		assertString(t, "ID", conn.ID, "conn-1")
		assertString(t, "Name", conn.Name, "Work VPN")
		assertString(t, "Host", conn.Host, "vpn.work.com")
		assertString(t, "Protocol", conn.Protocol, "anyconnect")
		assertString(t, "Username", conn.Username, "alice")
		assertBool(t, "HasPassword", conn.HasPassword, true)

		assertString(t, "DNS", clean.Settings.DNS, "8.8.8.8")
		assertBool(t, "Reconnect", clean.Settings.Reconnect, true)
		assertBool(t, "AutoCleanup", clean.Settings.AutoCleanup, true)
		assertString(t, "TunnelInterface", clean.Settings.TunnelInterface, "utun1")
		assertString(t, "NetInterface", clean.Settings.NetInterface, "en1")
		assertString(t, "WifiInterface", clean.Settings.WifiInterface, "Wi-Fi")
	})

	t.Run("drops invalid connections", func(t *testing.T) {
		cfg := models.Config{
			Connections: []models.Connection{
				{ID: "valid", Name: "Valid", Host: "vpn.valid.com"},
				{ID: "missing-name", Host: "vpn.example.com"},
				{Name: "missing-id", Host: "vpn.example.com"},
				{ID: "missing-host", Name: "Broken"},
			},
		}

		clean := sanitizeConfig(cfg)

		if len(clean.Connections) != 1 {
			t.Fatalf("expected 1 valid connection, got %d", len(clean.Connections))
		}
		assertString(t, "remaining ID", clean.Connections[0].ID, "valid")
	})

	t.Run("returns defaults for empty config", func(t *testing.T) {
		clean := sanitizeConfig(models.Config{})
		expected := models.NewConfig()

		if len(clean.Connections) != 0 {
			t.Fatalf("expected 0 connections, got %d", len(clean.Connections))
		}
		assertString(t, "DNS", clean.Settings.DNS, expected.Settings.DNS)
		assertBool(t, "Reconnect", clean.Settings.Reconnect, expected.Settings.Reconnect)
		assertBool(t, "AutoCleanup", clean.Settings.AutoCleanup, expected.Settings.AutoCleanup)
	})
}

func TestReadMsgDecode(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("{\"type\":\"hello_response\",\"version\":\"1.2.3\",\"compatible\":true}\n"))

	msg, err := ReadMsg(reader)
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	if msg.Type != "hello_response" {
		t.Fatalf("expected type hello_response, got %q", msg.Type)
	}

	var hello HelloResponse
	if err := msg.Decode(&hello); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	assertString(t, "Version", hello.Version, "1.2.3")
	assertBool(t, "Compatible", hello.Compatible, true)
}

func TestReadMsgErrors(t *testing.T) {
	t.Run("rejects malformed json", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("{oops}\n"))

		if _, err := ReadMsg(reader); err == nil {
			t.Fatal("expected error for malformed json")
		}
	})

	t.Run("rejects missing type", func(t *testing.T) {
		reader := bufio.NewReader(strings.NewReader("{\"version\":\"1.2.3\"}\n"))

		if _, err := ReadMsg(reader); err == nil {
			t.Fatal("expected error for missing type")
		}
	})
}

func TestIncomingMsgDecodeEmpty(t *testing.T) {
	msg := IncomingMsg{Type: "hello_response"}

	var hello HelloResponse
	if err := msg.Decode(&hello); err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestWriteMsgRoundTrip(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan error, 1)
	go func() {
		done <- WriteMsg(server, HelloResponse{
			Type:       "hello_response",
			Version:    "9.9.9",
			Compatible: true,
		})
	}()

	msg, err := ReadMsg(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("WriteMsg returned error: %v", err)
	}

	var hello HelloResponse
	if err := msg.Decode(&hello); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	assertString(t, "Type", hello.Type, "hello_response")
	assertString(t, "Version", hello.Version, "9.9.9")
	assertBool(t, "Compatible", hello.Compatible, true)
}

func TestHandleMessageInvalidConfigUpdate(t *testing.T) {
	d := newTestDaemon()
	original := d.state.Config

	d.handleMessage(IncomingMsg{
		Type: "config_update",
		raw:  json.RawMessage(`{"type":"config_update","config":"bad"}`),
	})

	if d.state.Config != original {
		t.Fatal("config should not change when decode fails")
	}
}

func TestHandleHelloVersionMismatch(t *testing.T) {
	d := newTestDaemon()
	d.version = "server-version"

	server, client := net.Pipe()
	defer client.Close()
	d.client = server
	defer server.Close()

	done := make(chan struct{})
	go func() {
		d.handleHello(HelloCmd{Type: "hello", Version: "client-version"})
		close(done)
	}()

	msg, err := ReadMsg(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	<-done

	var hello HelloResponse
	if err := msg.Decode(&hello); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	assertString(t, "Type", hello.Type, "hello_response")
	assertString(t, "Version", hello.Version, "server-version")
	assertBool(t, "Compatible", hello.Compatible, false)

	select {
	case <-d.shutdown:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected daemon shutdown after version mismatch")
	}
}

func TestShutdownIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "daemon.sock")
	if err := os.WriteFile(socketPath, []byte("socket"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	logFile, err := os.Create(filepath.Join(tmpDir, "vpn.log"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	listener := &mockListener{}
	d := newTestDaemon()
	d.listener = listener
	d.vpnLogFile = logFile
	d.socketPath = socketPath
	d.socketOwned = true

	d.Shutdown()
	d.Shutdown()

	select {
	case <-d.shutdown:
	default:
		t.Fatal("shutdown channel should be closed")
	}

	if listener.closeCount != 1 {
		t.Fatalf("listener Close called %d times, want 1", listener.closeCount)
	}
	if d.socketOwned {
		t.Fatal("socketOwned should be false after cleanup")
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("expected socket to be removed, got err=%v", err)
	}
	if _, err := logFile.WriteString("x"); err == nil {
		t.Fatal("expected vpn log file to be closed")
	}
}

func newTestDaemon() *Daemon {
	return &Daemon{
		state: &DaemonState{
			Status: StatusDisconnected,
			Config: models.NewConfig(),
		},
		shutdown:      make(chan struct{}),
		passwordCache: make(map[string]string),
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func attachTestClient(t *testing.T, d *Daemon) net.Conn {
	t.Helper()

	server, client := net.Pipe()
	d.client = server
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return client
}

func readTestMsg(t *testing.T, conn net.Conn) IncomingMsg {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline returned error: %v", err)
	}
	msg, err := ReadMsg(bufio.NewReader(conn))
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	return msg
}

type mockListener struct {
	mu         sync.Mutex
	closeCount int
}

func (l *mockListener) Accept() (net.Conn, error) {
	return nil, net.ErrClosed
}

func (l *mockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closeCount++
	return nil
}

func (l *mockListener) Addr() net.Addr {
	return mockAddr("test")
}

type mockAddr string

func (a mockAddr) Network() string {
	return "test"
}

func (a mockAddr) String() string {
	return string(a)
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

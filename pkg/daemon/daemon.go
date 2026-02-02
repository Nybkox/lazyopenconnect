package daemon

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/models"
	"github.com/Nybkox/lazyopenconnect/pkg/version"
)

const (
	MaxLogLines = 1000
	appName     = "lazyopenconnect"
)

type ConnStatus int

const (
	StatusDisconnected ConnStatus = iota
	StatusConnecting
	StatusPrompting
	StatusConnected
)

type DaemonState struct {
	Status       ConnStatus
	ActiveConnID string
	IP           string
	PID          int
	LogBuffer    []string
	Config       *models.Config
}

type Daemon struct {
	listener   net.Listener
	client     net.Conn
	clientMu   sync.Mutex
	state      *DaemonState
	vpnProcess *VPNProcess
	version    string
	shutdown   chan struct{}
	socketPath string
	pidPath    string
	logFile    *os.File
}

func SocketPath() (string, error) {
	dir, err := helpers.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.sock"), nil
}

func pidPath() (string, error) {
	dir, err := helpers.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.pid"), nil
}

func logPath() (string, error) {
	dir, err := helpers.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.log"), nil
}

func New() (*Daemon, error) {
	socketPath, err := SocketPath()
	if err != nil {
		return nil, err
	}

	pidFile, err := pidPath()
	if err != nil {
		return nil, err
	}

	return &Daemon{
		state: &DaemonState{
			Status:    StatusDisconnected,
			LogBuffer: make([]string, 0, MaxLogLines),
			Config:    models.NewConfig(),
		},
		version:    version.Current,
		shutdown:   make(chan struct{}),
		socketPath: socketPath,
		pidPath:    pidFile,
	}, nil
}

func Run() error {
	d, err := New()
	if err != nil {
		return err
	}

	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer d.closeLogging()

	log.Println("Daemon starting...")

	if err := d.killOldDaemon(); err != nil {
		log.Printf("Warning: failed to kill old daemon: %v", err)
	}

	exec.Command("pkill", "-9", "openconnect").Run()

	if err := d.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.removePID()

	if err := d.listen(); err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer d.Close()

	log.Printf("Daemon listening on %s (PID %d, version %s)", d.socketPath, os.Getpid(), d.version)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		d.Shutdown()
	}()

	d.acceptLoop()
	log.Println("Daemon stopped")
	return nil
}

func (d *Daemon) setupLogging() error {
	logFile, err := logPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	d.logFile = f
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return nil
}

func (d *Daemon) closeLogging() {
	if d.logFile != nil {
		d.logFile.Close()
	}
}

func (d *Daemon) killOldDaemon() error {
	data, err := os.ReadFile(d.pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	log.Printf("Killing old daemon (PID %d)", pid)
	process.Signal(syscall.SIGTERM)

	for i := 0; i < 20; i++ {
		if _, err := os.Stat(d.socketPath); os.IsNotExist(err) {
			log.Printf("Old daemon terminated, socket released")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Old daemon didn't release socket, sending SIGKILL")
	process.Signal(syscall.SIGKILL)
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (d *Daemon) writePID() error {
	if err := os.MkdirAll(filepath.Dir(d.pidPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(d.pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func (d *Daemon) removePID() {
	os.Remove(d.pidPath)
}

func (d *Daemon) listen() error {
	if err := os.MkdirAll(filepath.Dir(d.socketPath), 0o755); err != nil {
		return err
	}

	os.Remove(d.socketPath)

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return err
	}

	os.Chmod(d.socketPath, 0o600)
	d.listener = listener
	return nil
}

func (d *Daemon) acceptLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.shutdown:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}
		log.Printf("New client connected from %v", conn.RemoteAddr())
		d.handleNewClient(conn)
	}
}

func (d *Daemon) handleNewClient(conn net.Conn) {
	d.clientMu.Lock()
	if d.client != nil {
		log.Println("Kicking previous client")
		WriteMsg(d.client, KickedMsg{Type: "kicked"})
		d.client.Close()
	}
	d.client = conn
	d.clientMu.Unlock()

	go d.readLoop(conn)
}

func (d *Daemon) readLoop(conn net.Conn) {
	reader := bufio.NewReader(conn)

	for {
		msg, err := ReadMsg(reader)
		if err != nil {
			log.Printf("Read error: %v", err)
			d.clientMu.Lock()
			if d.client == conn {
				d.client.Close()
				d.client = nil
			}
			d.clientMu.Unlock()
			return
		}

		d.clientMu.Lock()
		if d.client != conn {
			log.Println("Connection superseded, exiting readLoop")
			d.clientMu.Unlock()
			return
		}
		d.clientMu.Unlock()

		d.handleMessage(msg)
	}
}

func (d *Daemon) handleMessage(msg map[string]any) {
	msgType, _ := msg["type"].(string)
	log.Printf("Received message: %s", msgType)

	switch msgType {
	case "hello":
		d.handleHello(msg)
	case "get_state":
		d.handleGetState()
	case "connect":
		d.handleConnect(msg)
	case "disconnect":
		d.handleDisconnect()
	case "input":
		d.handleInput(msg)
	case "config_update":
		d.handleConfigUpdate(msg)
	case "shutdown":
		d.Shutdown()
	default:
		log.Printf("Unknown message type: %s", msgType)
	}
}

func (d *Daemon) handleHello(msg map[string]any) {
	clientVersion, _ := msg["version"].(string)
	compatible := clientVersion == d.version

	log.Printf("Hello from client (version %s, compatible: %v)", clientVersion, compatible)

	d.sendToClient(HelloResponse{
		Type:       "hello_response",
		Version:    d.version,
		Compatible: compatible,
	})

	if !compatible {
		log.Printf("Version mismatch, shutting down daemon so client can spawn new one")
		go func() {
			time.Sleep(100 * time.Millisecond)
			d.Shutdown()
		}()
	}
}

func (d *Daemon) handleGetState() {
	log.Printf("Sending state: status=%d, connID=%s", d.state.Status, d.state.ActiveConnID)
	d.sendToClient(StateMsg{
		Type:         "state",
		Status:       int(d.state.Status),
		ActiveConnID: d.state.ActiveConnID,
		IP:           d.state.IP,
		PID:          d.state.PID,
		LogHistory:   d.state.LogBuffer,
	})
}

func (d *Daemon) handleConfigUpdate(msg map[string]any) {
	configData, ok := msg["config"].(map[string]any)
	if !ok {
		log.Println("Invalid config_update message")
		return
	}

	cfg := parseConfig(configData)
	d.state.Config = cfg
	log.Printf("Config updated: %d connections", len(cfg.Connections))
}

func parseConfig(data map[string]any) *models.Config {
	cfg := models.NewConfig()

	if conns, ok := data["connections"].([]any); ok {
		for _, c := range conns {
			if connMap, ok := c.(map[string]any); ok {
				conn := models.Connection{
					ID:          getString(connMap, "id"),
					Name:        getString(connMap, "name"),
					Protocol:    getString(connMap, "protocol"),
					Host:        getString(connMap, "host"),
					Username:    getString(connMap, "username"),
					HasPassword: getBool(connMap, "hasPassword"),
					Flags:       getString(connMap, "flags"),
				}
				cfg.Connections = append(cfg.Connections, conn)
			}
		}
	}

	if settings, ok := data["settings"].(map[string]any); ok {
		cfg.Settings.DNS = getString(settings, "dns")
		cfg.Settings.Reconnect = getBool(settings, "reconnect")
		cfg.Settings.AutoCleanup = getBool(settings, "autoCleanup")
		cfg.Settings.TunnelInterface = getString(settings, "tunnelInterface")
		cfg.Settings.NetInterface = getString(settings, "netInterface")
		cfg.Settings.WifiInterface = getString(settings, "wifiInterface")
	}

	return cfg
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func (d *Daemon) sendToClient(msg any) {
	d.clientMu.Lock()
	defer d.clientMu.Unlock()

	if d.client != nil {
		if err := WriteMsg(d.client, msg); err != nil {
			log.Printf("Failed to send message: %v", err)
		}
	}
}

func (d *Daemon) addLog(line string) {
	d.state.LogBuffer = append(d.state.LogBuffer, line)
	if len(d.state.LogBuffer) > MaxLogLines {
		d.state.LogBuffer = d.state.LogBuffer[1:]
	}
	d.sendToClient(LogMsg{Type: "log", Line: line})
}

func (d *Daemon) clearLogBuffer() {
	d.state.LogBuffer = d.state.LogBuffer[:0]
}

func (d *Daemon) Shutdown() {
	log.Println("Shutting down...")
	close(d.shutdown)
	if d.vpnProcess != nil {
		d.disconnectVPN()
	}
	if d.listener != nil {
		d.listener.Close()
	}
	os.Remove(d.socketPath)
}

func (d *Daemon) Close() {
	if d.listener != nil {
		d.listener.Close()
	}
	d.clientMu.Lock()
	if d.client != nil {
		d.client.Close()
	}
	d.clientMu.Unlock()
	os.Remove(d.socketPath)
}

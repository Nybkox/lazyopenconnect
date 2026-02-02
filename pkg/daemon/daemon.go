package daemon

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"os"
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
	LogLineCount int
	Config       *models.Config
}

type Daemon struct {
	listener   net.Listener
	client     net.Conn
	clientMu   sync.Mutex
	stateMu    sync.RWMutex
	vpnMu      sync.Mutex
	vpnLogMu   sync.Mutex
	state      *DaemonState
	vpnProcess *VPNProcess
	vpnLogFile *os.File
	version    string
	shutdown   chan struct{}
	socketPath string
	pidPath    string
	logFile    *os.File
	logger     *slog.Logger
	debug      bool
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

func New(debug bool) (*Daemon, error) {
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
			Status: StatusDisconnected,
			Config: models.NewConfig(),
		},
		version:    version.Current,
		shutdown:   make(chan struct{}),
		socketPath: socketPath,
		pidPath:    pidFile,
		debug:      debug,
	}, nil
}

func Run(debug bool) error {
	d, err := New(debug)
	if err != nil {
		return err
	}

	if err := d.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer d.closeLogging()

	d.logger.Info("daemon starting")

	if err := d.killOldDaemon(); err != nil {
		d.logger.Warn("failed to kill old daemon", "err", err)
	}

	if err := d.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.removePID()

	if err := d.listen(); err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer d.Close()

	d.logger.Info("daemon listening", "socket", d.socketPath, "pid", os.Getpid(), "version", d.version)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		d.logger.Info("signal received, shutting down", "signal", sig)
		d.Shutdown()
	}()

	d.acceptLoop()
	d.logger.Info("daemon stopped")
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

	level := slog.LevelInfo
	if d.debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	d.logger = slog.New(handler)
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

	d.logger.Info("killing old daemon", "pid", pid)
	process.Signal(syscall.SIGTERM)

	for i := 0; i < 20; i++ {
		if _, err := os.Stat(d.socketPath); os.IsNotExist(err) {
			d.logger.Debug("old daemon terminated, socket released")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	d.logger.Warn("old daemon didn't release socket, sending SIGKILL")
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
				d.logger.Error("accept failed", "err", err)
				continue
			}
		}
		d.logger.Info("client connected", "addr", conn.RemoteAddr())
		d.handleNewClient(conn)
	}
}

func (d *Daemon) handleNewClient(conn net.Conn) {
	d.clientMu.Lock()
	if d.client != nil {
		d.logger.Info("kicking previous client")
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
			d.logger.Debug("read error", "err", err)
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
			d.logger.Debug("connection superseded, exiting readLoop")
			d.clientMu.Unlock()
			return
		}
		d.clientMu.Unlock()

		d.handleMessage(msg)
	}
}

func (d *Daemon) handleMessage(msg map[string]any) {
	msgType, _ := msg["type"].(string)
	d.logger.Debug("message received", "type", msgType)

	switch msgType {
	case "hello":
		d.handleHello(msg)
	case "get_state":
		d.handleGetState()
	case "get_logs":
		d.handleGetLogs(msg)
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
		d.logger.Warn("unknown message type", "type", msgType)
	}
}

func (d *Daemon) handleHello(msg map[string]any) {
	clientVersion, _ := msg["version"].(string)
	compatible := clientVersion == d.version

	d.logger.Info("hello from client", "client_version", clientVersion, "compatible", compatible)

	d.sendToClient(HelloResponse{
		Type:       "hello_response",
		Version:    d.version,
		Compatible: compatible,
	})

	if !compatible {
		d.logger.Warn("version mismatch, shutting down daemon")
		go func() {
			time.Sleep(100 * time.Millisecond)
			d.Shutdown()
		}()
	}
}

func (d *Daemon) handleGetState() {
	d.stateMu.RLock()
	status := d.state.Status
	connID := d.state.ActiveConnID
	ip := d.state.IP
	pid := d.state.PID
	logLineCount := d.state.LogLineCount
	d.stateMu.RUnlock()

	d.logger.Debug("sending state", "status", status, "conn_id", connID)
	d.sendToClient(StateMsg{
		Type:          "state",
		Status:        int(status),
		ActiveConnID:  connID,
		IP:            ip,
		PID:           pid,
		TotalLogLines: logLineCount,
	})
}

func (d *Daemon) handleConfigUpdate(msg map[string]any) {
	configData, ok := msg["config"].(map[string]any)
	if !ok {
		d.logger.Warn("invalid config_update message")
		return
	}

	cfg := parseConfig(configData)
	d.stateMu.Lock()
	d.state.Config = cfg
	d.stateMu.Unlock()
	d.logger.Info("config updated", "connections", len(cfg.Connections))
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
			d.logger.Error("failed to send message", "err", err)
		}
	}
}

func (d *Daemon) addLog(line string) {
	d.vpnLogMu.Lock()
	if d.vpnLogFile != nil {
		d.vpnLogFile.WriteString(line + "\n")
	}
	d.vpnLogMu.Unlock()

	d.stateMu.Lock()
	lineNum := d.state.LogLineCount
	d.state.LogLineCount++
	d.stateMu.Unlock()

	d.sendToClient(LogMsg{Type: "log", Line: line, LineNumber: lineNum})
}

func vpnLogPath() (string, error) {
	dir, err := helpers.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vpn.log"), nil
}

func (d *Daemon) openVpnLogFile() error {
	path, err := vpnLogPath()
	if err != nil {
		return err
	}

	d.vpnLogMu.Lock()
	defer d.vpnLogMu.Unlock()

	if d.vpnLogFile != nil {
		d.vpnLogFile.Close()
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	d.vpnLogFile = f

	d.stateMu.Lock()
	d.state.LogLineCount = 0
	d.stateMu.Unlock()

	return nil
}

func (d *Daemon) closeVpnLogFile() {
	d.vpnLogMu.Lock()
	defer d.vpnLogMu.Unlock()

	if d.vpnLogFile != nil {
		d.vpnLogFile.Close()
		d.vpnLogFile = nil
	}
}

func (d *Daemon) readLogLines(from, to int) []string {
	path, err := vpnLogPath()
	if err != nil {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		if lineNum >= from && (to < 0 || lineNum < to) {
			lines = append(lines, scanner.Text())
		}
		lineNum++
		if to >= 0 && lineNum >= to {
			break
		}
	}

	return lines
}

func (d *Daemon) handleGetLogs(msg map[string]any) {
	from := 0
	to := -1

	if f, ok := msg["from"].(float64); ok {
		from = int(f)
	}
	if t, ok := msg["to"].(float64); ok {
		to = int(t)
	}

	d.stateMu.RLock()
	totalLines := d.state.LogLineCount
	d.stateMu.RUnlock()

	if from < 0 {
		from = 0
	}
	if to > totalLines {
		to = totalLines
	}

	lines := d.readLogLines(from, to)

	d.sendToClient(LogRangeMsg{
		Type:       "log_range",
		From:       from,
		Lines:      lines,
		TotalLines: totalLines,
	})
}

func (d *Daemon) Shutdown() {
	d.logger.Info("shutting down")
	close(d.shutdown)

	d.vpnMu.Lock()
	hasVPN := d.vpnProcess != nil
	d.vpnMu.Unlock()
	if hasVPN {
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

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

type ConnStatus = models.ConnStatus

const (
	StatusDisconnected = models.StatusDisconnected
	StatusConnecting   = models.StatusConnecting
	StatusPrompting    = models.StatusPrompting
	StatusConnected    = models.StatusConnected
	StatusExternal     = models.StatusExternal
	StatusReconnecting = models.StatusReconnecting
	StatusQuitting     = models.StatusQuitting
)

type DaemonState struct {
	Status          ConnStatus
	ActiveConnID    string
	IP              string
	PID             int
	LogLineCount    int
	Config          *models.Config
	NetworkSnapshot *helpers.NetworkSnapshot
	ExternalHost    string
	ExternalPID     int
}

type Daemon struct {
	listener    net.Listener
	client      net.Conn
	clientMu    sync.Mutex
	stateMu     sync.RWMutex
	vpnMu       sync.Mutex
	vpnLogMu    sync.Mutex
	state       *DaemonState
	vpnProcess  *VPNProcess
	vpnLogFile  *os.File
	version     string
	shutdown    chan struct{}
	socketPath  string
	pidPath     string
	lockPath    string
	logFile     *os.File
	lockFile    *os.File
	logger      *slog.Logger
	debug       bool
	socketOwned bool

	reconnectMu          sync.Mutex
	reconnecting         bool
	reconnectCancel      chan struct{}
	disconnectRequested  bool
	stoppingForReconnect bool
	passwordCache        map[string]string

	cleanupMu      sync.Mutex
	cleanupRunning bool
	shutdownOnce   sync.Once
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

func daemonLockPath() (string, error) {
	dir, err := helpers.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.lock"), nil
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

	lockFile, err := daemonLockPath()
	if err != nil {
		return nil, err
	}

	return &Daemon{
		state: &DaemonState{
			Status: StatusDisconnected,
			Config: models.NewConfig(),
		},
		version:       version.Current,
		shutdown:      make(chan struct{}),
		socketPath:    socketPath,
		pidPath:       pidFile,
		lockPath:      lockFile,
		debug:         debug,
		passwordCache: make(map[string]string),
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

	if err := d.acquireLock(); err != nil {
		return err
	}
	defer d.releaseLock()

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

	go d.wakeMonitor()
	go d.externalVPNMonitor()

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

const maxLogSize = 5 * 1024 * 1024 // 5MB

func (d *Daemon) setupLogging() error {
	logFile, err := logPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}

	rotateLogFile(logFile)

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

func rotateLogFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogSize {
		return
	}
	_ = os.Rename(path, path+".1")
}

func (d *Daemon) closeLogging() {
	if d.logFile != nil {
		d.logFile.Close()
	}
}

func (d *Daemon) acquireLock() error {
	if err := os.MkdirAll(filepath.Dir(d.lockPath), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(d.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("daemon already running")
		}
		return err
	}

	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
	}

	d.lockFile = f
	return nil
}

func (d *Daemon) releaseLock() {
	if d.lockFile == nil {
		return
	}
	_ = syscall.Flock(int(d.lockFile.Fd()), syscall.LOCK_UN)
	_ = d.lockFile.Close()
	d.lockFile = nil
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

	if err := d.prepareSocketPath(); err != nil {
		return err
	}

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return err
	}

	os.Chmod(d.socketPath, 0o600)
	if err := d.setSocketOwner(); err != nil {
		d.logger.Warn("failed to set socket owner", "err", err)
	}
	d.listener = listener
	d.socketOwned = true
	return nil
}

func (d *Daemon) setSocketOwner() error {
	uidStr := os.Getenv("SUDO_UID")
	gidStr := os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" {
		return nil
	}

	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return fmt.Errorf("invalid SUDO_UID: %w", err)
	}

	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return fmt.Errorf("invalid SUDO_GID: %w", err)
	}

	return os.Chown(d.socketPath, uid, gid)
}

func (d *Daemon) prepareSocketPath() error {
	info, err := os.Stat(d.socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("socket path exists and is not a socket")
	}

	conn, err := net.DialTimeout("unix", d.socketPath, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return fmt.Errorf("daemon already running")
	}

	return os.Remove(d.socketPath)
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

func decodeIncoming[T any](msg IncomingMsg) (T, error) {
	var decoded T
	err := msg.Decode(&decoded)
	return decoded, err
}

func (d *Daemon) handleMessage(msg IncomingMsg) {
	d.logger.Debug("message received", "type", msg.Type)

	switch msg.Type {
	case "hello":
		decoded, err := decodeIncoming[HelloCmd](msg)
		if err != nil {
			d.logger.Warn("invalid hello message", "err", err)
			return
		}
		d.handleHello(decoded)
	case "get_state":
		d.handleGetState()
	case "get_logs":
		decoded, err := decodeIncoming[GetLogsCmd](msg)
		if err != nil {
			d.logger.Warn("invalid get_logs message", "err", err)
			return
		}
		d.handleGetLogs(decoded)
	case "clear_logs":
		d.handleClearLogs()
	case "connect":
		decoded, err := decodeIncoming[ConnectCmd](msg)
		if err != nil {
			d.logger.Warn("invalid connect message", "err", err)
			return
		}
		d.handleConnect(decoded)
	case "disconnect":
		d.handleDisconnect()
	case "input":
		decoded, err := decodeIncoming[InputCmd](msg)
		if err != nil {
			d.logger.Warn("invalid input message", "err", err)
			return
		}
		d.handleInput(decoded)
	case "config_update":
		decoded, err := decodeIncoming[ConfigUpdateCmd](msg)
		if err != nil {
			d.logger.Warn("invalid config_update message", "err", err)
			return
		}
		d.handleConfigUpdate(decoded)
	case "cleanup":
		d.cleanupMu.Lock()
		if d.cleanupRunning {
			d.cleanupMu.Unlock()
			d.sendToClient(ErrorMsg{Type: "error", Code: "cleanup_running", Message: "cleanup already running"})
			return
		}
		d.cleanupRunning = true
		d.cleanupMu.Unlock()
		go d.handleCleanup()

	case "shutdown":
		d.Shutdown()
	default:
		d.logger.Warn("unknown message type", "type", msg.Type)
	}
}

func (d *Daemon) handleHello(msg HelloCmd) {
	compatible := msg.Version == d.version

	d.logger.Info("hello from client", "client_version", msg.Version, "compatible", compatible)

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
	externalHost := d.state.ExternalHost
	d.stateMu.RUnlock()

	d.logger.Debug("sending state", "status", status, "conn_id", connID)
	d.sendToClient(StateMsg{
		Type:          "state",
		Status:        int(status),
		ActiveConnID:  connID,
		IP:            ip,
		PID:           pid,
		TotalLogLines: logLineCount,
		ExternalHost:  externalHost,
	})
}

func (d *Daemon) handleConfigUpdate(msg ConfigUpdateCmd) {
	cfg := sanitizeConfig(msg.Config)
	d.stateMu.Lock()
	d.state.Config = cfg
	d.stateMu.Unlock()
	d.logger.Info("config updated", "connections", len(cfg.Connections))
}

func sanitizeConfig(cfg models.Config) *models.Config {
	if len(cfg.Connections) == 0 && cfg.Settings == (models.Settings{}) {
		return models.NewConfig()
	}

	clean := models.NewConfig()
	clean.Settings = cfg.Settings
	clean.Connections = make([]models.Connection, 0, len(cfg.Connections))

	for _, conn := range cfg.Connections {
		if conn.ID == "" || conn.Name == "" || conn.Host == "" {
			continue
		}
		clean.Connections = append(clean.Connections, conn)
	}

	return clean
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

func (d *Daemon) resetVpnLogFile() error {
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

func (d *Daemon) ensureVpnLogFile() error {
	path, err := vpnLogPath()
	if err != nil {
		return err
	}

	d.vpnLogMu.Lock()
	defer d.vpnLogMu.Unlock()

	if d.vpnLogFile != nil {
		return nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	d.vpnLogFile = f

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

func (d *Daemon) handleGetLogs(msg GetLogsCmd) {
	from := msg.From
	to := msg.To

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

func (d *Daemon) handleClearLogs() {
	if err := d.resetVpnLogFile(); err != nil {
		d.sendToClient(ErrorMsg{Type: "error", Code: "clear_logs_failed", Message: err.Error()})
		return
	}
	d.handleGetState()
}

func (d *Daemon) cleanupSnapshot() *helpers.NetworkSnapshot {
	d.stateMu.RLock()
	snap := d.state.NetworkSnapshot
	settings := d.state.Config.Settings
	d.stateMu.RUnlock()

	if snap == nil {
		snap = helpers.CaptureNetworkSnapshot()
	}
	if settings.NetInterface != "" {
		snap.DefaultInterface = settings.NetInterface
	}
	if settings.WifiInterface != "" {
		snap.WifiServiceName = settings.WifiInterface
	}
	if settings.DNS != "" {
		snap.DNSServers = strings.Fields(settings.DNS)
	}
	if settings.TunnelInterface != "" {
		snap.TunnelInterface = settings.TunnelInterface
	}

	return snap
}

func (d *Daemon) runCleanup(label string) {
	d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: fmt.Sprintf("--- %s ---", label)})

	results := helpers.RunCleanupSteps(d.cleanupSnapshot())
	for _, line := range helpers.FormatCleanupResults(results) {
		d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: line})
	}

	d.sendToClient(CleanupDoneMsg{Type: "cleanup_done"})
}

func (d *Daemon) handleCleanup() {
	defer func() {
		d.cleanupMu.Lock()
		d.cleanupRunning = false
		d.cleanupMu.Unlock()
	}()

	d.logger.Info("running manual cleanup")
	d.runCleanup("Running cleanup")
}

func (d *Daemon) runCleanupSync(label string) {
	d.cleanupMu.Lock()
	if d.cleanupRunning {
		d.cleanupMu.Unlock()
		return
	}
	d.cleanupRunning = true
	d.cleanupMu.Unlock()

	defer func() {
		d.cleanupMu.Lock()
		d.cleanupRunning = false
		d.cleanupMu.Unlock()
	}()

	d.logger.Info("running cleanup", "label", label)
	d.runCleanup(label)
}

func (d *Daemon) runAutoCleanup() {
	go d.runCleanupSync("Auto-cleanup")
}

func (d *Daemon) Shutdown() {
	d.shutdownOnce.Do(func() {
		if d.logger != nil {
			d.logger.Info("shutting down")
		}
		close(d.shutdown)

		d.vpnMu.Lock()
		hasVPN := d.vpnProcess != nil
		d.vpnMu.Unlock()
		if hasVPN {
			d.disconnectVPN()
		}

		if d.listener != nil {
			_ = d.listener.Close()
		}
		d.closeVpnLogFile()
		d.cleanupSocket()
	})
}

func (d *Daemon) Close() {
	if d.listener != nil {
		_ = d.listener.Close()
	}
	d.clientMu.Lock()
	if d.client != nil {
		_ = d.client.Close()
	}
	d.clientMu.Unlock()
	d.closeVpnLogFile()
	d.cleanupSocket()
}

func (d *Daemon) cleanupSocket() {
	if !d.socketOwned {
		return
	}
	_ = os.Remove(d.socketPath)
	d.socketOwned = false
}

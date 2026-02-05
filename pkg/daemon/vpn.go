package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

var (
	ipPattern  = regexp.MustCompile(`Configured as (\d+\.\d+\.\d+\.\d+)`)
	pidPattern = regexp.MustCompile(`pid (\d+)`)

	connectedPatterns = []string{
		"continuing in background",
		"configured as",
		"established dtls connection",
		"dtls established",
		"ssl established",
		"tunnel is up",
		"session authentication will expire",
	}
)

type VPNProcess struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func (d *Daemon) handleConnect(msg map[string]any) {
	connID, _ := msg["conn_id"].(string)
	password, _ := msg["password"].(string)

	d.cancelReconnect()

	d.stateMu.Lock()
	if d.state.Status != StatusDisconnected {
		d.stateMu.Unlock()
		d.logger.Warn("connect rejected, already connected", "status", d.state.Status)
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "already_connected",
			Message: "Already connected or connecting",
		})
		return
	}

	var conn *models.Connection
	for i := range d.state.Config.Connections {
		if d.state.Config.Connections[i].ID == connID {
			conn = &d.state.Config.Connections[i]
			break
		}
	}

	if conn == nil {
		d.stateMu.Unlock()
		d.logger.Warn("connect rejected, connection not found", "conn_id", connID)
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "invalid_conn",
			Message: "Connection not found",
		})
		return
	}

	d.state.Status = StatusConnecting
	d.state.ActiveConnID = connID
	d.stateMu.Unlock()

	d.reconnectMu.Lock()
	d.disconnectRequested = false
	if password != "" && conn.HasPassword {
		d.passwordCache[connID] = password
	}
	d.reconnectMu.Unlock()

	d.logger.Info("connecting", "conn_id", connID, "host", conn.Host, "protocol", conn.Protocol)
	if err := d.resetVpnLogFile(); err != nil {
		d.logger.Error("failed to open vpn log file", "err", err)
	}

	go d.startVPN(conn, password)
}

func (d *Daemon) startVPN(conn *models.Connection, password string) {
	args := buildArgs(conn)

	cmdStr := "openconnect " + strings.Join(args, " ")
	d.addLog(ui.LogCommand(cmdStr))
	d.logger.Debug("executing openconnect", "args", args)

	cmd := exec.Command("openconnect", args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		d.logger.Error("pty start failed", "err", err)
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "start_failed",
			Message: err.Error(),
		})
		d.sendToClient(DisconnectedMsg{Type: "disconnected"})
		d.stateMu.Lock()
		d.state.Status = StatusDisconnected
		d.state.ActiveConnID = ""
		d.stateMu.Unlock()
		return
	}

	_, err = term.MakeRaw(int(ptmx.Fd()))
	if err != nil {
		d.logger.Error("pty make raw failed", "err", err)
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "pty_failed",
			Message: err.Error(),
		})
		d.sendToClient(DisconnectedMsg{Type: "disconnected"})
		d.stateMu.Lock()
		d.state.Status = StatusDisconnected
		d.state.ActiveConnID = ""
		d.stateMu.Unlock()
		return
	}

	d.vpnMu.Lock()
	d.vpnProcess = &VPNProcess{
		cmd:  cmd,
		ptmx: ptmx,
	}
	d.vpnMu.Unlock()

	pid := cmd.Process.Pid
	d.stateMu.Lock()
	d.state.PID = pid
	d.stateMu.Unlock()

	d.logger.Info("vpn process started", "pid", pid)

	if password != "" {
		d.logger.Debug("sending password to stdin")
		go func() {
			time.Sleep(100 * time.Millisecond)
			ptmx.Write([]byte(password + "\n"))
		}()
	}

	d.streamPTYOutput(ptmx)
}

func buildArgs(conn *models.Connection) []string {
	args := []string{
		"--protocol=" + conn.Protocol,
		conn.Host,
	}

	if conn.Username != "" {
		args = append(args, "--user="+conn.Username)
	}

	if conn.HasPassword {
		args = append(args, "--passwd-on-stdin")
	}

	if conn.Flags != "" {
		flags := strings.Fields(conn.Flags)
		args = append(args, flags...)
	}

	return args
}

func (d *Daemon) streamPTYOutput(ptmx *os.File) {
	buf := make([]byte, 1024)
	var lineBuf strings.Builder

	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			if lineBuf.Len() > 0 {
				d.addLog(lineBuf.String())
			}
			d.handleVPNExit()
			return
		}

		for i := 0; i < n; i++ {
			ch := buf[i]
			if ch == '\n' || ch == '\r' {
				if lineBuf.Len() > 0 {
					line := lineBuf.String()
					d.addLog(line)
					d.checkLineForEvents(line)
					lineBuf.Reset()
				}
			} else {
				lineBuf.WriteByte(ch)
			}
		}

		partial := lineBuf.String()
		if isPrompt(partial) {
			d.addLog(partial)
			d.sendPrompt(partial)
			lineBuf.Reset()
		}
	}
}

func isPrompt(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}

	if !strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, "?") {
		return false
	}

	lower := strings.ToLower(trimmed)

	promptKeywords := []string{
		"password", "passwd", "passcode",
		"username", "user",
		"token", "otp", "code",
		"response", "answer",
		"enter", "input",
		"login", "credential",
		"secret", "pin",
	}

	for _, kw := range promptKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	if len(trimmed) < 60 {
		return true
	}

	return false
}

func isPasswordPrompt(line string) bool {
	lower := strings.ToLower(line)
	sensitiveKeywords := []string{
		"password", "passwd", "passcode",
		"secret", "key", "token",
		"pin", "otp",
		"credential",
	}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (d *Daemon) sendPrompt(line string) {
	isPassword := isPasswordPrompt(line)
	d.logger.Debug("prompt detected", "is_password", isPassword)
	d.stateMu.Lock()
	d.state.Status = StatusPrompting
	d.stateMu.Unlock()
	d.sendToClient(PromptMsg{
		Type:       "prompt",
		IsPassword: isPassword,
	})
}

func (d *Daemon) checkLineForEvents(line string) {
	ip := ""
	pid := 0

	if match := ipPattern.FindStringSubmatch(line); len(match) > 1 {
		ip = match[1]
	}

	if match := pidPattern.FindStringSubmatch(line); len(match) > 1 {
		pid, _ = strconv.Atoi(match[1])
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range connectedPatterns {
		if strings.Contains(lineLower, pattern) {
			d.stateMu.Lock()
			d.state.Status = StatusConnected
			if ip != "" {
				d.state.IP = ip
			}
			if pid != 0 {
				d.state.PID = pid
			}
			currentIP := d.state.IP
			currentPID := d.state.PID
			d.stateMu.Unlock()
			d.logger.Info("vpn connected", "ip", currentIP, "pid", currentPID, "pattern", pattern)
			d.sendToClient(ConnectedMsg{
				Type: "connected",
				IP:   currentIP,
				PID:  currentPID,
			})
			break
		}
	}
}

func (d *Daemon) handleVPNExit() {
	d.vpnMu.Lock()
	d.vpnProcess = nil
	d.vpnMu.Unlock()

	d.reconnectMu.Lock()
	stoppingForReconnect := d.stoppingForReconnect
	d.stoppingForReconnect = false
	disconnectRequested := d.disconnectRequested
	d.reconnectMu.Unlock()

	if stoppingForReconnect {
		d.logger.Debug("vpn exit due to reconnect, skipping cleanup")
		return
	}

	d.stateMu.Lock()
	wasConnected := d.state.Status == StatusConnected
	wasConnecting := d.state.Status == StatusConnecting
	wasPrompting := d.state.Status == StatusPrompting
	autoCleanup := d.state.Config.Settings.AutoCleanup
	reconnectEnabled := d.state.Config.Settings.Reconnect
	connID := d.state.ActiveConnID
	d.state.Status = StatusDisconnected
	d.state.ActiveConnID = ""
	d.state.IP = ""
	d.state.PID = 0
	d.stateMu.Unlock()

	d.logger.Info("vpn process exited", "was_connected", wasConnected, "disconnect_requested", disconnectRequested)

	shouldReconnect := reconnectEnabled && !disconnectRequested && connID != "" &&
		(wasConnected || wasConnecting || wasPrompting)

	if shouldReconnect {
		d.logger.Info("vpn exit: initiating auto-reconnect", "conn_id", connID)
		d.addLog(ui.LogWarning("--- Connection lost, will reconnect ---"))
		go d.startAutoReconnect(connID, "exit")
		return
	}

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})

	if wasConnected && autoCleanup {
		d.logger.Debug("running auto cleanup")
		d.runCleanup()
	}
}

func (d *Daemon) handleDisconnect() {
	d.cancelReconnect()

	d.stateMu.RLock()
	status := d.state.Status
	d.stateMu.RUnlock()

	if status == StatusDisconnected {
		return
	}

	d.disconnectVPN()
}

func (d *Daemon) disconnectVPN() {
	d.logger.Info("disconnecting vpn")

	d.vpnMu.Lock()
	proc := d.vpnProcess
	d.vpnProcess = nil
	d.vpnMu.Unlock()

	if proc != nil && proc.cmd != nil && proc.cmd.Process != nil {
		pid := proc.cmd.Process.Pid
		d.addLog(ui.LogCommand(fmt.Sprintf("kill %d", pid)))
		proc.cmd.Process.Signal(syscall.SIGTERM)

		go func() {
			time.Sleep(3 * time.Second)
			proc.cmd.Process.Kill()
		}()

		if proc.ptmx != nil {
			proc.ptmx.Close()
		}
	}

	d.stateMu.Lock()
	d.state.Status = StatusDisconnected
	d.state.ActiveConnID = ""
	d.state.IP = ""
	d.state.PID = 0
	autoCleanup := d.state.Config.Settings.AutoCleanup
	d.stateMu.Unlock()

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})

	if autoCleanup {
		d.logger.Debug("running auto cleanup")
		d.runCleanup()
	}
}

func (d *Daemon) handleInput(msg map[string]any) {
	value, _ := msg["value"].(string)

	d.vpnMu.Lock()
	proc := d.vpnProcess
	d.vpnMu.Unlock()

	if proc != nil && proc.ptmx != nil {
		d.logger.Debug("sending input to vpn")
		proc.ptmx.Write([]byte(value + "\n"))
		d.stateMu.Lock()
		d.state.Status = StatusConnecting
		d.stateMu.Unlock()
	}
}

func (d *Daemon) runCleanup() {
	d.logger.Info("starting cleanup")
	d.stateMu.RLock()
	settings := d.state.Config.Settings
	d.stateMu.RUnlock()

	tunnelIface := settings.GetTunnelInterface()
	netIface := settings.GetNetInterface()
	wifiIface := settings.GetWifiInterface()
	dns := settings.GetDNS()

	steps := []struct {
		name string
		cmd  string
		fn   func() error
	}{
		{
			"Killing tunnel interface (" + tunnelIface + ")",
			"ifconfig " + tunnelIface + " down",
			func() error {
				return exec.Command("ifconfig", tunnelIface, "down").Run()
			},
		},
		{
			"Flushing routes",
			"route -n flush",
			func() error {
				return exec.Command("route", "-n", "flush").Run()
			},
		},
		{
			"Restarting network interface (" + netIface + ")",
			"ifconfig " + netIface + " down && ifconfig " + netIface + " up",
			func() error {
				exec.Command("ifconfig", netIface, "down").Run()
				time.Sleep(500 * time.Millisecond)
				return exec.Command("ifconfig", netIface, "up").Run()
			},
		},
		{
			"Restoring DNS to " + dns,
			"networksetup -setdnsservers " + wifiIface + " " + dns,
			func() error {
				args := append([]string{"-setdnsservers", wifiIface}, strings.Fields(dns)...)
				return exec.Command("networksetup", args...).Run()
			},
		},
		{
			"Flushing DNS cache",
			"dscacheutil -flushcache && killall -HUP mDNSResponder",
			func() error {
				exec.Command("dscacheutil", "-flushcache").Run()
				return exec.Command("killall", "-HUP", "mDNSResponder").Run()
			},
		},
	}

	for _, step := range steps {
		d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: step.name + "..."})
		d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: ui.LogCommand(step.cmd)})
		if err := step.fn(); err != nil {
			d.logger.Debug("cleanup step failed", "step", step.name, "err", err)
			d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: ui.LogFail("✗ " + err.Error())})
		} else {
			d.logger.Debug("cleanup step completed", "step", step.name)
			d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: ui.LogOK("✓ Done")})
		}
	}

	d.logger.Info("cleanup completed")
	d.sendToClient(CleanupDoneMsg{Type: "cleanup_done"})
}

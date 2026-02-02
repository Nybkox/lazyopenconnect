package daemon

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
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

	if d.state.Status != StatusDisconnected {
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
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "invalid_conn",
			Message: "Connection not found",
		})
		return
	}

	d.state.Status = StatusConnecting
	d.state.ActiveConnID = connID
	d.clearLogBuffer()

	go d.startVPN(conn, password)
}

func (d *Daemon) startVPN(conn *models.Connection, password string) {
	args := buildArgs(conn)

	cmdStr := "openconnect " + strings.Join(args, " ")
	d.addLog("\x1b[36m$ " + cmdStr + "\x1b[0m")

	cmd := exec.Command("openconnect", args...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "start_failed",
			Message: err.Error(),
		})
		d.state.Status = StatusDisconnected
		d.state.ActiveConnID = ""
		return
	}

	_, err = term.MakeRaw(int(ptmx.Fd()))
	if err != nil {
		d.sendToClient(ErrorMsg{
			Type:    "error",
			Code:    "pty_failed",
			Message: err.Error(),
		})
		d.state.Status = StatusDisconnected
		d.state.ActiveConnID = ""
		return
	}

	d.vpnProcess = &VPNProcess{
		cmd:  cmd,
		ptmx: ptmx,
	}
	d.state.PID = cmd.Process.Pid

	if password != "" {
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
	d.state.Status = StatusPrompting
	d.sendToClient(PromptMsg{
		Type:       "prompt",
		IsPassword: isPasswordPrompt(line),
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
			d.state.Status = StatusConnected
			if ip != "" {
				d.state.IP = ip
			}
			if pid != 0 {
				d.state.PID = pid
			}
			d.sendToClient(ConnectedMsg{
				Type: "connected",
				IP:   d.state.IP,
				PID:  d.state.PID,
			})
			break
		}
	}
}

func (d *Daemon) handleVPNExit() {
	wasConnected := d.state.Status == StatusConnected

	d.vpnProcess = nil
	d.state.Status = StatusDisconnected
	d.state.ActiveConnID = ""
	d.state.IP = ""
	d.state.PID = 0

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})

	if wasConnected && d.state.Config.Settings.AutoCleanup {
		d.runCleanup()
	}
}

func (d *Daemon) handleDisconnect() {
	if d.state.Status == StatusDisconnected {
		return
	}

	d.disconnectVPN()
}

func (d *Daemon) disconnectVPN() {
	d.addLog("\x1b[36m$ pkill -x openconnect\x1b[0m")
	exec.Command("pkill", "-x", "openconnect").Run()

	if d.vpnProcess != nil && d.vpnProcess.ptmx != nil {
		d.vpnProcess.ptmx.Close()
	}

	d.vpnProcess = nil
	d.state.Status = StatusDisconnected
	d.state.ActiveConnID = ""
	d.state.IP = ""
	d.state.PID = 0

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})

	if d.state.Config.Settings.AutoCleanup {
		d.runCleanup()
	}
}

func (d *Daemon) handleInput(msg map[string]any) {
	value, _ := msg["value"].(string)

	if d.vpnProcess != nil && d.vpnProcess.ptmx != nil {
		d.vpnProcess.ptmx.Write([]byte(value + "\n"))
		d.state.Status = StatusConnecting
	}
}

func (d *Daemon) runCleanup() {
	settings := &d.state.Config.Settings
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
		d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: "\x1b[36m$ " + step.cmd + "\x1b[0m"})
		if err := step.fn(); err != nil {
			d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: "  \x1b[31m✗ " + err.Error() + "\x1b[0m"})
		} else {
			d.sendToClient(CleanupStepMsg{Type: "cleanup_step", Line: "  \x1b[32m✓ Done\x1b[0m"})
		}
	}

	d.sendToClient(CleanupDoneMsg{Type: "cleanup_done"})
}

package daemon

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

const (
	wakeTickInterval     = 5 * time.Second
	wakeThreshold        = 20
	wakeDebounce         = 10 * time.Second
	networkWaitInitial   = 3 * time.Second
	networkProbeMax      = 30 * time.Second
	maxReconnectAttempts = 3
)

var reconnectBackoffs = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

func (d *Daemon) wakeMonitor() {
	ticker := time.NewTicker(wakeTickInterval)
	defer ticker.Stop()

	lastWall := time.Now().Unix()
	var lastWake time.Time

	for {
		select {
		case <-d.shutdown:
			return
		case <-ticker.C:
			now := time.Now()
			nowWall := now.Unix()
			delta := nowWall - lastWall
			lastWall = nowWall

			if delta > wakeThreshold {
				if time.Since(lastWake) < wakeDebounce {
					d.logger.Debug("wake detected but debounced", "delta", delta)
					continue
				}
				lastWake = now
				d.logger.Info("wake detected", "sleep_duration_sec", delta)
				d.handleWake()
			}
		}
	}
}

func (d *Daemon) handleWake() {
	d.stateMu.RLock()
	status := d.state.Status
	connID := d.state.ActiveConnID
	reconnectEnabled := d.state.Config.Settings.Reconnect
	d.stateMu.RUnlock()

	if connID == "" {
		d.logger.Debug("wake: no active connection")
		return
	}

	if !reconnectEnabled {
		d.logger.Debug("wake: reconnect disabled in settings")
		return
	}

	if status != StatusConnected && status != StatusConnecting && status != StatusPrompting {
		d.logger.Debug("wake: not in reconnectable state", "status", status)
		return
	}

	d.logger.Info("wake: initiating reconnect", "conn_id", connID)
	d.addLog(ui.LogWarning("--- Wake detected, will reconnect ---"))

	d.stopForReconnect()
	go d.startAutoReconnect(connID, "wake")
}

func (d *Daemon) stopForReconnect() {
	d.reconnectMu.Lock()
	d.stoppingForReconnect = true
	d.reconnectMu.Unlock()

	d.vpnMu.Lock()
	proc := d.vpnProcess
	d.vpnMu.Unlock()

	if proc == nil {
		d.reconnectMu.Lock()
		d.stoppingForReconnect = false
		d.reconnectMu.Unlock()
		return
	}

	d.logger.Debug("stopping vpn for reconnect")

	if proc.cmd != nil && proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
	if proc.ptmx != nil {
		proc.ptmx.Close()
	}

	d.vpnMu.Lock()
	d.vpnProcess = nil
	d.vpnMu.Unlock()

	d.stateMu.Lock()
	d.state.Status = StatusDisconnected
	d.state.IP = ""
	d.state.PID = 0
	d.stateMu.Unlock()
}

func (d *Daemon) startAutoReconnect(connID string, reason string) {
	d.reconnectMu.Lock()
	if d.reconnecting {
		d.reconnectMu.Unlock()
		d.logger.Debug("reconnect already in progress")
		return
	}
	d.reconnecting = true
	d.reconnectCancel = make(chan struct{})
	d.disconnectRequested = false
	cancelCh := d.reconnectCancel
	d.reconnectMu.Unlock()

	defer func() {
		d.reconnectMu.Lock()
		d.reconnecting = false
		d.reconnectCancel = nil
		d.reconnectMu.Unlock()
	}()

	d.stateMu.RLock()
	var conn *models.Connection
	for i := range d.state.Config.Connections {
		if d.state.Config.Connections[i].ID == connID {
			conn = &d.state.Config.Connections[i]
			break
		}
	}
	d.stateMu.RUnlock()

	if conn == nil {
		d.logger.Warn("reconnect: connection not found", "conn_id", connID)
		d.addLog(ui.LogError("Reconnect failed: connection not found"))
		return
	}

	d.sendToClient(ReconnectingMsg{
		Type:    "reconnecting",
		ConnID:  connID,
		Reason:  reason,
		Attempt: 0,
		Max:     maxReconnectAttempts,
	})

	d.addLog(ui.LogWarning("Waiting for network..."))
	d.logger.Debug("waiting initial delay before network probe")

	select {
	case <-cancelCh:
		d.logger.Debug("reconnect cancelled during initial wait")
		return
	case <-d.shutdown:
		return
	case <-time.After(networkWaitInitial):
	}

	if !d.waitForNetwork(conn.Host, cancelCh) {
		d.logger.Warn("reconnect: network not available")
		d.addLog(ui.LogError("Reconnect failed: network not available"))
		d.stateMu.Lock()
		d.state.Status = StatusDisconnected
		d.state.ActiveConnID = ""
		d.state.IP = ""
		d.state.PID = 0
		d.stateMu.Unlock()
		d.sendToClient(DisconnectedMsg{Type: "disconnected"})
		return
	}

	d.addLog(ui.LogOK("Network available"))

	password := ""
	if conn.HasPassword {
		d.reconnectMu.Lock()
		password = d.passwordCache[connID]
		d.reconnectMu.Unlock()
	}

	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		select {
		case <-cancelCh:
			d.logger.Debug("reconnect cancelled")
			return
		case <-d.shutdown:
			return
		default:
		}

		d.reconnectMu.Lock()
		if d.disconnectRequested {
			d.reconnectMu.Unlock()
			d.logger.Debug("reconnect cancelled: disconnect requested")
			return
		}
		d.reconnectMu.Unlock()

		d.logger.Info("reconnect attempt", "attempt", attempt, "max", maxReconnectAttempts)
		d.addLog(ui.LogWarning(fmt.Sprintf("Reconnecting... (attempt %d/%d)", attempt, maxReconnectAttempts)))

		d.sendToClient(ReconnectingMsg{
			Type:    "reconnecting",
			ConnID:  connID,
			Reason:  reason,
			Attempt: attempt,
			Max:     maxReconnectAttempts,
		})

		d.stateMu.Lock()
		d.state.Status = StatusConnecting
		d.state.ActiveConnID = connID
		d.stateMu.Unlock()

		d.doConnect(connID, password)

		connectTimeout := 30 * time.Second
		connectDeadline := time.Now().Add(connectTimeout)

		for time.Now().Before(connectDeadline) {
			select {
			case <-cancelCh:
				return
			case <-d.shutdown:
				return
			case <-time.After(500 * time.Millisecond):
			}

			d.stateMu.RLock()
			status := d.state.Status
			d.stateMu.RUnlock()

			if status == StatusConnected {
				d.logger.Info("reconnect successful")
				d.addLog(ui.LogOK("Reconnected successfully"))
				return
			}

			if status == StatusPrompting {
				d.logger.Info("reconnect waiting for user input (prompt)")
				return
			}

			if status == StatusDisconnected {
				d.logger.Debug("reconnect attempt failed, vpn exited")
				break
			}
		}

		d.stateMu.RLock()
		status := d.state.Status
		d.stateMu.RUnlock()

		if status == StatusConnected || status == StatusPrompting {
			return
		}

		if attempt < maxReconnectAttempts {
			backoff := reconnectBackoffs[attempt-1]
			d.addLog(ui.LogWarning(fmt.Sprintf("Retrying in %ds...", int(backoff.Seconds()))))
			select {
			case <-cancelCh:
				return
			case <-d.shutdown:
				return
			case <-time.After(backoff):
			}
		}
	}

	d.logger.Warn("reconnect: all attempts failed")
	d.addLog(ui.LogError("All reconnect attempts failed"))

	d.stateMu.Lock()
	d.state.Status = StatusDisconnected
	d.state.ActiveConnID = ""
	d.stateMu.Unlock()

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})
}

func (d *Daemon) waitForNetwork(host string, cancel <-chan struct{}) bool {
	probeAddr := extractProbeAddr(host)
	d.logger.Debug("probing network", "addr", probeAddr)

	deadline := time.Now().Add(networkProbeMax)
	for time.Now().Before(deadline) {
		select {
		case <-cancel:
			return false
		case <-d.shutdown:
			return false
		default:
		}

		conn, err := net.DialTimeout("tcp", probeAddr, 2*time.Second)
		if err == nil {
			conn.Close()
			return true
		}

		d.logger.Debug("network probe failed", "err", err)

		select {
		case <-cancel:
			return false
		case <-d.shutdown:
			return false
		case <-time.After(1 * time.Second):
		}
	}

	return false
}

func extractProbeAddr(host string) string {
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil {
			host = u.Host
		}
	}

	if strings.Contains(host, ":") {
		return host
	}

	return host + ":443"
}

func (d *Daemon) doConnect(connID, password string) {
	d.stateMu.RLock()
	var conn *models.Connection
	for i := range d.state.Config.Connections {
		if d.state.Config.Connections[i].ID == connID {
			conn = &d.state.Config.Connections[i]
			break
		}
	}
	d.stateMu.RUnlock()

	if conn == nil {
		return
	}

	if err := d.ensureVpnLogFile(); err != nil {
		d.logger.Error("failed to open vpn log file", "err", err)
	}

	go d.startVPN(conn, password)
}

func (d *Daemon) cancelReconnect() {
	d.reconnectMu.Lock()
	defer d.reconnectMu.Unlock()

	d.disconnectRequested = true
	if d.reconnectCancel != nil {
		select {
		case <-d.reconnectCancel:
		default:
			close(d.reconnectCancel)
		}
	}
}

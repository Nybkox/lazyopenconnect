package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const externalCheckInterval = 5 * time.Second

func (d *Daemon) externalVPNMonitor() {
	ticker := time.NewTicker(externalCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdown:
			return
		case <-ticker.C:
			d.checkExternalVPN()
		}
	}
}

func (d *Daemon) checkExternalVPN() {
	d.stateMu.RLock()
	status := d.state.Status
	d.stateMu.RUnlock()

	if status != StatusDisconnected && status != StatusExternal {
		d.clearExternal()
		return
	}

	d.vpnMu.Lock()
	var ownVPNPID int
	if d.vpnProcess != nil && d.vpnProcess.cmd != nil && d.vpnProcess.cmd.Process != nil {
		ownVPNPID = d.vpnProcess.cmd.Process.Pid
	}
	d.vpnMu.Unlock()

	pid, host := d.findExternalOpenconnect(ownVPNPID)

	d.stateMu.Lock()
	if pid != 0 {
		changed := d.state.Status != StatusExternal || d.state.ExternalPID != pid
		d.state.Status = StatusExternal
		d.state.ExternalHost = host
		d.state.ExternalPID = pid
		d.state.PID = pid
		d.stateMu.Unlock()
		if changed {
			d.logger.Info("external openconnect detected", "pid", pid, "host", host)
			d.broadcastState()
		}
	} else if d.state.Status == StatusExternal {
		d.state.Status = StatusDisconnected
		d.state.ExternalHost = ""
		d.state.ExternalPID = 0
		d.state.PID = 0
		d.stateMu.Unlock()
		d.logger.Info("external openconnect gone")
		d.broadcastState()
	} else {
		d.stateMu.Unlock()
	}
}

func (d *Daemon) clearExternal() {
	d.stateMu.Lock()
	if d.state.ExternalHost != "" {
		d.state.ExternalHost = ""
		d.state.ExternalPID = 0
	}
	d.stateMu.Unlock()
}

func (d *Daemon) broadcastState() {
	d.handleGetState()
}

func (d *Daemon) findExternalOpenconnect(ownVPNPID int) (int, string) {
	out, err := exec.Command("ps", "-axo", "pid=,comm=,args=").Output()
	if err != nil {
		return 0, ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		comm := fields[1]
		if !strings.HasSuffix(comm, "openconnect") {
			continue
		}

		if pid == ownVPNPID && ownVPNPID != 0 {
			continue
		}

		host := extractHostFromArgs(fields[2:])
		return pid, host
	}

	return 0, ""
}

func (d *Daemon) killExternalVPN(pid int) {
	if pid == 0 {
		return
	}

	d.logger.Info("killing external openconnect", "pid", pid)

	proc, err := os.FindProcess(pid)
	if err != nil {
		d.logger.Warn("could not find external process", "pid", pid, "err", err)
		return
	}

	proc.Signal(syscall.SIGTERM)

	d.stateMu.Lock()
	d.state.Status = StatusDisconnected
	d.state.ExternalHost = ""
	d.state.ExternalPID = 0
	d.state.PID = 0
	d.stateMu.Unlock()

	d.sendToClient(DisconnectedMsg{Type: "disconnected"})
}

func extractHostFromArgs(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "openconnect" {
			continue
		}
		return arg
	}
	return ""
}

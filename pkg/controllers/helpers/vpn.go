package helpers

import (
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

type (
	VPNLogMsg    string
	VPNPromptMsg struct {
		IsPassword bool
	}
	VPNConnectedMsg struct {
		IP  string
		PID int
	}
	VPNErrorMsg   error
	VPNStdinReady struct {
		Stdin io.WriteCloser
		PID   int
	}
	VPNDisconnectedMsg     struct{}
	VPNCleanupStepMsg      string
	VPNCleanupDoneMsg      struct{}
	VPNExternalDetectedMsg struct {
		PID  int
		Host string
	}
	VPNNoExternalMsg   struct{}
	VPNProcessDiedMsg  struct{}
	VPNProcessAliveMsg struct{}
)

var (
	ipPattern  = regexp.MustCompile(`Configured as (\d+\.\d+\.\d+\.\d+)`)
	pidPattern = regexp.MustCompile(`pid (\d+)`)
)

type VPNProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

func StartVPN(conn *models.Connection, password string) tea.Cmd {
	return func() tea.Msg {
		args := buildArgs(conn)

		cmdStr := "openconnect " + strings.Join(args, " ")
		LogChan <- "\x1b[36m$ " + cmdStr + "\x1b[0m"

		cmd := exec.Command("openconnect", args...)

		ptmx, err := pty.Start(cmd)
		if err != nil {
			return VPNErrorMsg(err)
		}

		pid := cmd.Process.Pid

		go streamPTYOutput(ptmx)

		if password != "" {
			go func() {
				time.Sleep(100 * time.Millisecond)
				ptmx.Write([]byte(password + "\n"))
			}()
		}

		return VPNStdinReady{Stdin: ptmx, PID: pid}
	}
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

var (
	LogChan         = make(chan string, 100)
	PromptChan      = make(chan VPNPromptMsg, 10)
	ConnectedChan   = make(chan VPNConnectedMsg, 1)
	DisconnectChan  = make(chan struct{}, 1)
	CleanupStepChan = make(chan string, 20)
)

var lastPromptLine string

func streamPTYOutput(ptmx *os.File) {
	buf := make([]byte, 1024)
	var lineBuf strings.Builder

	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			if lineBuf.Len() > 0 {
				LogChan <- lineBuf.String()
			}
			return
		}

		for i := 0; i < n; i++ {
			ch := buf[i]
			if ch == '\n' || ch == '\r' {
				if lineBuf.Len() > 0 {
					line := lineBuf.String()
					LogChan <- line
					checkLineForEvents(line)
					lineBuf.Reset()
				}
			} else {
				lineBuf.WriteByte(ch)
			}
		}

		partial := lineBuf.String()
		if isPrompt(partial) {
			LogChan <- partial
			sendPrompt(partial)
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

func sendPrompt(line string) {
	lastPromptLine = line
	PromptChan <- VPNPromptMsg{IsPassword: isPasswordPrompt(line)}
}

func checkLineForEvents(line string) {
	if isPrompt(line) {
		sendPrompt(line)
	}

	ip := ""
	pid := 0

	if match := ipPattern.FindStringSubmatch(line); len(match) > 1 {
		ip = match[1]
	}

	if match := pidPattern.FindStringSubmatch(line); len(match) > 1 {
		pid, _ = strconv.Atoi(match[1])
	}

	if strings.Contains(line, "Continuing in background") ||
		(strings.Contains(line, "Configured as") && ip != "") ||
		strings.Contains(line, "DTLS established") ||
		strings.Contains(line, "SSL established") ||
		strings.Contains(line, "Tunnel is up") {

		select {
		case ConnectedChan <- VPNConnectedMsg{IP: ip, PID: pid}:
		default:
		}
	}
}

func WaitForLog() tea.Cmd {
	return func() tea.Msg {
		return VPNLogMsg(<-LogChan)
	}
}

func WaitForPrompt() tea.Cmd {
	return func() tea.Msg {
		return <-PromptChan
	}
}

func WaitForConnected() tea.Cmd {
	return func() tea.Msg {
		return <-ConnectedChan
	}
}

func CheckExternalVPN() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("pgrep", "-x", "openconnect").Output()
		if err != nil {
			return VPNNoExternalMsg{}
		}
		pidStr := strings.TrimSpace(string(out))
		if idx := strings.Index(pidStr, "\n"); idx > 0 {
			pidStr = pidStr[:idx]
		}
		pid, _ := strconv.Atoi(pidStr)
		if pid > 0 {
			host := parseExternalHost(pid)
			return VPNExternalDetectedMsg{PID: pid, Host: host}
		}
		return VPNNoExternalMsg{}
	}
}

func parseExternalHost(pid int) string {
	out, err := exec.Command("ps", "-ww", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err != nil {
		return ""
	}

	args := strings.Fields(strings.TrimSpace(string(out)))
	for i := len(args) - 1; i >= 0; i-- {
		arg := args[i]
		if arg == "openconnect" || strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func CheckProcessAlive(pid int) tea.Cmd {
	return func() tea.Msg {
		if pid <= 0 {
			return VPNProcessDiedMsg{}
		}

		out, err := exec.Command("pgrep", "-x", "openconnect").Output()
		if err != nil {
			return VPNProcessDiedMsg{}
		}

		pids := strings.Fields(strings.TrimSpace(string(out)))
		pidStr := strconv.Itoa(pid)
		for _, p := range pids {
			if p == pidStr {
				return VPNProcessAliveMsg{}
			}
		}

		return VPNProcessDiedMsg{}
	}
}

func DisconnectVPN() tea.Cmd {
	return func() tea.Msg {
		LogChan <- "\x1b[36m$ pkill -x openconnect\x1b[0m"
		exec.Command("pkill", "-x", "openconnect").Run()
		return VPNDisconnectedMsg{}
	}
}

func RunCleanup(settings *models.Settings) tea.Cmd {
	return func() tea.Msg {
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
			CleanupStepChan <- step.name + "..."
			CleanupStepChan <- "\x1b[36m$ " + step.cmd + "\x1b[0m"
			if err := step.fn(); err != nil {
				CleanupStepChan <- "  \x1b[31m✗ " + err.Error() + "\x1b[0m"
			} else {
				CleanupStepChan <- "  \x1b[32m✓ Done\x1b[0m"
			}
		}

		return VPNCleanupDoneMsg{}
	}
}

func WaitForCleanupStep() tea.Cmd {
	return func() tea.Msg {
		return VPNCleanupStepMsg(<-CleanupStepChan)
	}
}

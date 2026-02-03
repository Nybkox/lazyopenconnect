package helpers

import (
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

type VPNCleanupStepMsg string
type VPNCleanupDoneMsg struct{}

var CleanupStepChan = make(chan string, 20)

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
			CleanupStepChan <- ui.LogCommand(step.cmd)
			if err := step.fn(); err != nil {
				CleanupStepChan <- ui.LogFail("✗ " + err.Error())
			} else {
				CleanupStepChan <- ui.LogOK("✓ Done")
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

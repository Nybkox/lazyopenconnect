//go:build darwin

package helpers

import (
	"strings"
	"time"
)

func PlatformCleanupSteps(snap *NetworkSnapshot) []CleanupStep {
	tunnelIface := snap.TunnelInterface
	if tunnelIface == "" {
		tunnelIface = "utun0"
	}

	netIface := snap.DefaultInterface
	if netIface == "" {
		netIface = "en0"
	}

	wifiSvc := snap.WifiServiceName
	if wifiSvc == "" {
		wifiSvc = "Wi-Fi"
	}

	dns := strings.Join(snap.DNSServers, " ")
	if dns == "" {
		dns = "1.1.1.1 1.0.0.1"
	}

	var steps []CleanupStep

	steps = append(steps, CleanupStep{
		Name: "Killing tunnel interface (" + tunnelIface + ")",
		Cmd:  "ifconfig " + tunnelIface + " down",
		Fn: func() error {
			return runCmd("ifconfig", tunnelIface, "down")
		},
	})

	steps = append(steps, CleanupStep{
		Name: "Flushing routes",
		Cmd:  "route -n flush",
		Fn: func() error {
			return runCmd("route", "-n", "flush")
		},
	})

	steps = append(steps, CleanupStep{
		Name: "Restarting network interface (" + netIface + ")",
		Cmd:  "ifconfig " + netIface + " down && ifconfig " + netIface + " up",
		Fn: func() error {
			runCmd("ifconfig", netIface, "down")
			time.Sleep(500 * time.Millisecond)
			return runCmd("ifconfig", netIface, "up")
		},
	})

	dnsArgs := strings.Fields(dns)
	steps = append(steps, CleanupStep{
		Name: "Restoring DNS to " + dns,
		Cmd:  "networksetup -setdnsservers " + wifiSvc + " " + dns,
		Fn: func() error {
			args := append([]string{"-setdnsservers", wifiSvc}, dnsArgs...)
			return runCmd("networksetup", args...)
		},
	})

	steps = append(steps, CleanupStep{
		Name: "Flushing DNS cache",
		Cmd:  "dscacheutil -flushcache && killall -HUP mDNSResponder",
		Fn: func() error {
			runCmd("dscacheutil", "-flushcache")
			return runCmd("killall", "-HUP", "mDNSResponder")
		},
	})

	return steps
}

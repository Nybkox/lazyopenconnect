//go:build linux

package helpers

import (
	"os"
	"strings"
)

func PlatformCleanupSteps(snap *NetworkSnapshot) []CleanupStep {
	tunnelIface := snap.TunnelInterface
	if tunnelIface == "" {
		tunnelIface = "tun0"
	}

	netIface := snap.DefaultInterface
	if netIface == "" {
		netIface = "eth0"
	}

	gateway := snap.DefaultGateway
	dns := strings.Join(snap.DNSServers, " ")

	var steps []CleanupStep

	steps = append(steps, CleanupStep{
		Name: "Killing tunnel interface (" + tunnelIface + ")",
		Cmd:  "ip link set " + tunnelIface + " down",
		Fn: func() error {
			return runCmd("ip", "link", "set", tunnelIface, "down")
		},
	})

	steps = append(steps, CleanupStep{
		Name: "Flushing VPN routes (" + tunnelIface + ")",
		Cmd:  "ip route flush dev " + tunnelIface,
		Fn: func() error {
			return runCmd("ip", "route", "flush", "dev", tunnelIface)
		},
	})

	if gateway != "" {
		steps = append(steps, CleanupStep{
			Name: "Restoring default route via " + gateway,
			Cmd:  "ip route add default via " + gateway + " dev " + netIface,
			Fn: func() error {
				_ = runCmd("ip", "route", "del", "default")
				return runCmd("ip", "route", "add", "default", "via", gateway, "dev", netIface)
			},
		})
	}

	if dns != "" {
		if isSystemdResolved() {
			dnsServers := strings.Fields(dns)
			args := append([]string{"dns", netIface}, dnsServers...)
			steps = append(steps, CleanupStep{
				Name: "Restoring DNS to " + dns,
				Cmd:  "resolvectl dns " + netIface + " " + dns,
				Fn: func() error {
					return runCmd("resolvectl", args...)
				},
			})

			steps = append(steps, CleanupStep{
				Name: "Flushing DNS cache",
				Cmd:  "resolvectl flush-caches",
				Fn: func() error {
					return runCmd("resolvectl", "flush-caches")
				},
			})
		} else {
			steps = append(steps, CleanupStep{
				Name: "Restoring DNS to " + dns,
				Cmd:  "write /etc/resolv.conf",
				Fn: func() error {
					return writeResolvConf(strings.Fields(dns))
				},
			})
		}
	}

	return steps
}

func writeResolvConf(servers []string) error {
	var sb strings.Builder
	for _, s := range servers {
		sb.WriteString("nameserver " + s + "\n")
	}
	return os.WriteFile("/etc/resolv.conf", []byte(sb.String()), 0o644)
}

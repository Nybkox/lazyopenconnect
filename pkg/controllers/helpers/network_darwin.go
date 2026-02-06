//go:build darwin

package helpers

import (
	"os/exec"
	"strings"
)

func DetectDefaultInterface() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "interface:")), nil
		}
	}
	return "", nil
}

func DetectWifiServiceName(iface string) (string, error) {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if strings.Contains(line, "Device: "+iface) && i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			if strings.HasPrefix(prev, "Hardware Port:") {
				return strings.TrimSpace(strings.TrimPrefix(prev, "Hardware Port:")), nil
			}
		}
	}
	return "", nil
}

func DetectDNSServers(serviceName string) ([]string, error) {
	out, err := exec.Command("networksetup", "-getdnsservers", serviceName).Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 && !strings.Contains(lines[0], "aren't any") {
			var servers []string
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" {
					servers = append(servers, l)
				}
			}
			if len(servers) > 0 {
				return servers, nil
			}
		}
	}

	return parseDNSFromResolvConf()
}

func DetectDefaultGateway() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gateway:")), nil
		}
	}
	return "", nil
}

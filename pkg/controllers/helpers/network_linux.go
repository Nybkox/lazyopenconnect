//go:build linux

package helpers

import (
	"os/exec"
	"strings"
)

func DetectDefaultInterface() (string, error) {
	out, err := exec.Command("ip", "-o", "route", "show", "default").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", nil
}

func DetectWifiServiceName(iface string) (string, error) {
	return iface, nil
}

func DetectDNSServers(_ string) ([]string, error) {
	if isSystemdResolved() {
		out, err := exec.Command("resolvectl", "dns").Output()
		if err == nil {
			return parseResolvectlDNS(string(out)), nil
		}
	}
	return parseDNSFromResolvConf()
}

func DetectDefaultGateway() (string, error) {
	out, err := exec.Command("ip", "-o", "route", "show", "default").Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", nil
}

func isSystemdResolved() bool {
	_, err := exec.LookPath("resolvectl")
	return err == nil
}

func parseResolvectlDNS(output string) []string {
	var servers []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Global:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		for _, s := range strings.Fields(parts[1]) {
			s = strings.TrimSpace(s)
			if s != "" {
				servers = append(servers, s)
			}
		}
	}
	return servers
}

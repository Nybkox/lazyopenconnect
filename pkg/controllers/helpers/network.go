package helpers

import (
	"os"
	"strings"
)

type NetworkSnapshot struct {
	DefaultInterface string   `json:"default_interface"`
	WifiServiceName  string   `json:"wifi_service_name"`
	DNSServers       []string `json:"dns_servers"`
	DefaultGateway   string   `json:"default_gateway"`
	TunnelInterface  string   `json:"tunnel_interface"`
}

func CaptureNetworkSnapshot() *NetworkSnapshot {
	snap := &NetworkSnapshot{}

	if iface, err := DetectDefaultInterface(); err == nil && iface != "" {
		snap.DefaultInterface = iface
	}

	if snap.DefaultInterface != "" {
		if svc, err := DetectWifiServiceName(snap.DefaultInterface); err == nil && svc != "" {
			snap.WifiServiceName = svc
		}
	}

	svcName := snap.WifiServiceName
	if svcName == "" {
		svcName = snap.DefaultInterface
	}
	if servers, err := DetectDNSServers(svcName); err == nil && len(servers) > 0 {
		snap.DNSServers = servers
	}

	if gw, err := DetectDefaultGateway(); err == nil && gw != "" {
		snap.DefaultGateway = gw
	}

	return snap
}

func parseDNSFromResolvConf() ([]string, error) {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	var servers []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] != "127.0.0.53" {
				servers = append(servers, fields[1])
			}
		}
	}
	return servers, nil
}

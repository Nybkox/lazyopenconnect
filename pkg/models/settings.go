package models

import "runtime"

type Settings struct {
	DNS               string `json:"dns"`
	Reconnect         bool   `json:"reconnect"`
	AutoCleanup       bool   `json:"autoCleanup"`
	WifiInterface     string `json:"wifiInterface"`
	NetInterface      string `json:"netInterface"`
	TunnelInterface   string `json:"tunnelInterface"`
	SkipVersionUpdate string `json:"skipVersionUpdate"`
}

func (s *Settings) GetWifiInterface() string {
	if s.WifiInterface == "" {
		return "Wi-Fi"
	}
	return s.WifiInterface
}

func (s *Settings) GetNetInterface() string {
	if s.NetInterface != "" {
		return s.NetInterface
	}
	if runtime.GOOS == "darwin" {
		return "en0"
	}
	return "eth0"
}

func (s *Settings) GetTunnelInterface() string {
	if s.TunnelInterface != "" {
		return s.TunnelInterface
	}
	if runtime.GOOS == "darwin" {
		return "utun0"
	}
	return "tun0"
}

func (s *Settings) GetDNS() string {
	if s.DNS == "" {
		return "1.1.1.1 1.0.0.1"
	}
	return s.DNS
}

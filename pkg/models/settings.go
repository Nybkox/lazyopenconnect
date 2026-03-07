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

func DefaultWifiInterface() string {
	return "Wi-Fi"
}

func DefaultNetInterface() string {
	if runtime.GOOS == "darwin" {
		return "en0"
	}
	return "eth0"
}

func DefaultTunnelInterface() string {
	if runtime.GOOS == "darwin" {
		return "utun0"
	}
	return "tun0"
}

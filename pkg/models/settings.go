package models

type Settings struct {
	DNS               string `json:"dns"`
	Reconnect         bool   `json:"reconnect"`
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
	if s.NetInterface == "" {
		return "en0"
	}
	return s.NetInterface
}

func (s *Settings) GetTunnelInterface() string {
	if s.TunnelInterface == "" {
		return "utun0"
	}
	return s.TunnelInterface
}

func (s *Settings) GetDNS() string {
	if s.DNS == "" {
		return "1.1.1.1 1.0.0.1"
	}
	return s.DNS
}

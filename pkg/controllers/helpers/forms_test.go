package helpers

import (
	"testing"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func TestNormalizedValue(t *testing.T) {
	if got := normalizedValue("  value \t"); got != "value" {
		t.Fatalf("normalizedValue returned %q, want %q", got, "value")
	}

	if got := normalizedValue("   "); got != "" {
		t.Fatalf("normalizedValue returned %q, want empty string", got)
	}
}

func TestSettingsFormDataToSettings(t *testing.T) {
	data := &SettingsFormData{
		DNS:             " 1.1.1.1 8.8.8.8 ",
		Reconnect:       true,
		AutoCleanup:     true,
		WifiInterface:   "   ",
		NetInterface:    " en7 ",
		TunnelInterface: " utun9 ",
	}

	settings := data.ToSettings()

	if settings.DNS != "1.1.1.1 8.8.8.8" {
		t.Fatalf("DNS = %q, want %q", settings.DNS, "1.1.1.1 8.8.8.8")
	}
	if !settings.Reconnect {
		t.Fatal("Reconnect should be true")
	}
	if !settings.AutoCleanup {
		t.Fatal("AutoCleanup should be true")
	}
	if settings.WifiInterface != "" {
		t.Fatalf("WifiInterface = %q, want empty string", settings.WifiInterface)
	}
	if settings.NetInterface != "en7" {
		t.Fatalf("NetInterface = %q, want %q", settings.NetInterface, "en7")
	}
	if settings.TunnelInterface != "utun9" {
		t.Fatalf("TunnelInterface = %q, want %q", settings.TunnelInterface, "utun9")
	}
}

func TestNewConnectionFormDataNilDefaultsProtocol(t *testing.T) {
	data := NewConnectionFormData(nil)

	if data.Protocol != "fortinet" {
		t.Fatalf("Protocol = %q, want %q", data.Protocol, "fortinet")
	}
}

func TestConnectionFormDataToConnectionKeepsExistingPassword(t *testing.T) {
	data := &ConnectionFormData{
		Name:     "Work",
		Protocol: "anyconnect",
		Host:     "vpn.example.com",
		Username: "alice",
		Password: "   ",
	}
	existing := &models.Connection{ID: "conn-1", HasPassword: true}

	conn := data.ToConnection(existing)

	if conn.ID != "conn-1" {
		t.Fatalf("ID = %q, want %q", conn.ID, "conn-1")
	}
	if !conn.HasPassword {
		t.Fatal("HasPassword should remain true when password input is blank")
	}
}

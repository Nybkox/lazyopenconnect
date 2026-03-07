package daemon

import (
	"bufio"
	"strings"
	"testing"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func TestSanitizeConfig(t *testing.T) {
	t.Run("keeps valid connections and settings", func(t *testing.T) {
		cfg := models.Config{
			Connections: []models.Connection{
				{ID: "conn-1", Name: "Work VPN", Host: "vpn.work.com", Protocol: "anyconnect", Username: "alice", HasPassword: true},
			},
			Settings: models.Settings{
				DNS:             "8.8.8.8",
				Reconnect:       true,
				AutoCleanup:     true,
				TunnelInterface: "utun1",
				NetInterface:    "en1",
				WifiInterface:   "Wi-Fi",
			},
		}

		clean := sanitizeConfig(cfg)

		if len(clean.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(clean.Connections))
		}
		conn := clean.Connections[0]
		assertString(t, "ID", conn.ID, "conn-1")
		assertString(t, "Name", conn.Name, "Work VPN")
		assertString(t, "Host", conn.Host, "vpn.work.com")
		assertString(t, "Protocol", conn.Protocol, "anyconnect")
		assertString(t, "Username", conn.Username, "alice")
		assertBool(t, "HasPassword", conn.HasPassword, true)

		assertString(t, "DNS", clean.Settings.DNS, "8.8.8.8")
		assertBool(t, "Reconnect", clean.Settings.Reconnect, true)
		assertBool(t, "AutoCleanup", clean.Settings.AutoCleanup, true)
		assertString(t, "TunnelInterface", clean.Settings.TunnelInterface, "utun1")
		assertString(t, "NetInterface", clean.Settings.NetInterface, "en1")
		assertString(t, "WifiInterface", clean.Settings.WifiInterface, "Wi-Fi")
	})

	t.Run("drops invalid connections", func(t *testing.T) {
		cfg := models.Config{
			Connections: []models.Connection{
				{ID: "valid", Name: "Valid", Host: "vpn.valid.com"},
				{ID: "missing-name", Host: "vpn.example.com"},
				{Name: "missing-id", Host: "vpn.example.com"},
				{ID: "missing-host", Name: "Broken"},
			},
		}

		clean := sanitizeConfig(cfg)

		if len(clean.Connections) != 1 {
			t.Fatalf("expected 1 valid connection, got %d", len(clean.Connections))
		}
		assertString(t, "remaining ID", clean.Connections[0].ID, "valid")
	})

	t.Run("returns defaults for empty config", func(t *testing.T) {
		clean := sanitizeConfig(models.Config{})
		expected := models.NewConfig()

		if len(clean.Connections) != 0 {
			t.Fatalf("expected 0 connections, got %d", len(clean.Connections))
		}
		assertString(t, "DNS", clean.Settings.DNS, expected.Settings.DNS)
		assertBool(t, "Reconnect", clean.Settings.Reconnect, expected.Settings.Reconnect)
		assertBool(t, "AutoCleanup", clean.Settings.AutoCleanup, expected.Settings.AutoCleanup)
	})
}

func TestReadMsgDecode(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("{\"type\":\"hello_response\",\"version\":\"1.2.3\",\"compatible\":true}\n"))

	msg, err := ReadMsg(reader)
	if err != nil {
		t.Fatalf("ReadMsg returned error: %v", err)
	}
	if msg.Type != "hello_response" {
		t.Fatalf("expected type hello_response, got %q", msg.Type)
	}

	var hello HelloResponse
	if err := msg.Decode(&hello); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	assertString(t, "Version", hello.Version, "1.2.3")
	assertBool(t, "Compatible", hello.Compatible, true)
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

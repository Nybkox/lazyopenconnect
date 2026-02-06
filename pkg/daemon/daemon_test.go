package daemon

import (
	"testing"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"existing key", map[string]any{"host": "vpn.example.com"}, "host", "vpn.example.com"},
		{"missing key", map[string]any{"host": "vpn.example.com"}, "name", ""},
		{"wrong type", map[string]any{"port": 443}, "port", ""},
		{"nil value", map[string]any{"host": nil}, "host", ""},
		{"empty string", map[string]any{"host": ""}, "host", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getString(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getString(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want bool
	}{
		{"true value", map[string]any{"reconnect": true}, "reconnect", true},
		{"false value", map[string]any{"reconnect": false}, "reconnect", false},
		{"missing key", map[string]any{}, "reconnect", false},
		{"wrong type string", map[string]any{"reconnect": "true"}, "reconnect", false},
		{"wrong type int", map[string]any{"reconnect": 1}, "reconnect", false},
		{"nil value", map[string]any{"reconnect": nil}, "reconnect", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBool(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getBool(%v, %q) = %v, want %v", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	t.Run("full valid input", func(t *testing.T) {
		data := map[string]any{
			"connections": []any{
				map[string]any{
					"id":          "conn-1",
					"name":        "Work VPN",
					"protocol":    "anyconnect",
					"host":        "vpn.work.com",
					"username":    "alice",
					"hasPassword": true,
					"flags":       "--no-dtls",
				},
			},
			"settings": map[string]any{
				"dns":             "8.8.8.8",
				"reconnect":       true,
				"tunnelInterface": "utun1",
				"netInterface":    "en1",
				"wifiInterface":   "Wi-Fi",
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
		}
		conn := cfg.Connections[0]
		assertString(t, "ID", conn.ID, "conn-1")
		assertString(t, "Name", conn.Name, "Work VPN")
		assertString(t, "Protocol", conn.Protocol, "anyconnect")
		assertString(t, "Host", conn.Host, "vpn.work.com")
		assertString(t, "Username", conn.Username, "alice")
		assertBool(t, "HasPassword", conn.HasPassword, true)
		assertString(t, "Flags", conn.Flags, "--no-dtls")

		assertString(t, "DNS", cfg.Settings.DNS, "8.8.8.8")
		assertBool(t, "Reconnect", cfg.Settings.Reconnect, true)
		assertString(t, "TunnelInterface", cfg.Settings.TunnelInterface, "utun1")
		assertString(t, "NetInterface", cfg.Settings.NetInterface, "en1")
		assertString(t, "WifiInterface", cfg.Settings.WifiInterface, "Wi-Fi")
	})

	t.Run("empty map returns default config", func(t *testing.T) {
		cfg := parseConfig(map[string]any{})
		expected := models.NewConfig()

		if len(cfg.Connections) != len(expected.Connections) {
			t.Errorf("expected %d connections, got %d", len(expected.Connections), len(cfg.Connections))
		}
		assertString(t, "DNS", cfg.Settings.DNS, expected.Settings.DNS)
		assertBool(t, "Reconnect", cfg.Settings.Reconnect, expected.Settings.Reconnect)
	})

	t.Run("connections only", func(t *testing.T) {
		data := map[string]any{
			"connections": []any{
				map[string]any{
					"id":   "c1",
					"name": "Test",
					"host": "vpn.test.com",
				},
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
		}
		assertString(t, "Host", cfg.Connections[0].Host, "vpn.test.com")
		assertString(t, "DNS", cfg.Settings.DNS, "")
		assertBool(t, "Reconnect", cfg.Settings.Reconnect, false)
	})

	t.Run("settings only", func(t *testing.T) {
		data := map[string]any{
			"settings": map[string]any{
				"dns":       "1.1.1.1",
				"reconnect": true,
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 0 {
			t.Errorf("expected 0 connections, got %d", len(cfg.Connections))
		}
		assertString(t, "DNS", cfg.Settings.DNS, "1.1.1.1")
		assertBool(t, "Reconnect", cfg.Settings.Reconnect, true)
	})

	t.Run("malformed connection entry skipped", func(t *testing.T) {
		data := map[string]any{
			"connections": []any{
				"not-a-map",
				map[string]any{"id": "valid", "name": "Valid"},
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 1 {
			t.Fatalf("expected 1 connection (malformed skipped), got %d", len(cfg.Connections))
		}
		assertString(t, "ID", cfg.Connections[0].ID, "valid")
	})

	t.Run("missing fields in connection default to zero values", func(t *testing.T) {
		data := map[string]any{
			"connections": []any{
				map[string]any{"id": "partial"},
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
		}
		conn := cfg.Connections[0]
		assertString(t, "ID", conn.ID, "partial")
		assertString(t, "Name", conn.Name, "")
		assertString(t, "Host", conn.Host, "")
		assertBool(t, "HasPassword", conn.HasPassword, false)
	})

	t.Run("multiple connections", func(t *testing.T) {
		data := map[string]any{
			"connections": []any{
				map[string]any{"id": "c1", "name": "First"},
				map[string]any{"id": "c2", "name": "Second"},
				map[string]any{"id": "c3", "name": "Third"},
			},
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 3 {
			t.Fatalf("expected 3 connections, got %d", len(cfg.Connections))
		}
		assertString(t, "first ID", cfg.Connections[0].ID, "c1")
		assertString(t, "second ID", cfg.Connections[1].ID, "c2")
		assertString(t, "third ID", cfg.Connections[2].ID, "c3")
	})

	t.Run("connections wrong type ignored", func(t *testing.T) {
		data := map[string]any{
			"connections": "not-a-slice",
		}

		cfg := parseConfig(data)

		if len(cfg.Connections) != 0 {
			t.Errorf("expected 0 connections, got %d", len(cfg.Connections))
		}
	})

	t.Run("settings wrong type ignored", func(t *testing.T) {
		data := map[string]any{
			"settings": "not-a-map",
		}

		cfg := parseConfig(data)

		assertString(t, "DNS", cfg.Settings.DNS, "")
		assertBool(t, "Reconnect", cfg.Settings.Reconnect, false)
	})
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

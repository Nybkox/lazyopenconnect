package helpers

import (
	"errors"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

var errRequired = errors.New("required")

func formTheme() *huh.Theme {
	return huh.ThemeCharm()
}

type ConnectionFormData struct {
	Name     string
	Protocol string
	Host     string
	Username string
	Password string
	Flags    string
}

func NewConnectionFormData(conn *models.Connection) *ConnectionFormData {
	if conn == nil {
		return &ConnectionFormData{
			Protocol: "fortinet",
		}
	}
	return &ConnectionFormData{
		Name:     conn.Name,
		Protocol: conn.Protocol,
		Host:     conn.Host,
		Username: conn.Username,
		Flags:    conn.Flags,
	}
}

func (d *ConnectionFormData) ToConnection(existing *models.Connection) *models.Connection {
	passwordProvided := strings.TrimSpace(d.Password) != ""
	conn := &models.Connection{
		Name:        d.Name,
		Protocol:    d.Protocol,
		Host:        d.Host,
		Username:    d.Username,
		HasPassword: passwordProvided,
		Flags:       d.Flags,
	}
	if existing != nil {
		conn.ID = existing.ID
		if !passwordProvided {
			conn.HasPassword = existing.HasPassword
		}
	}
	return conn
}

func NewConnectionForm(data *ConnectionFormData, width int, isEdit bool) *huh.Form {
	title := "New Connection"
	if isEdit {
		title = "Edit Connection"
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Prompt("> ").
				Value(&data.Name).
				Validate(func(s string) error {
					if s == "" {
						return errRequired
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("Protocol").
				Options(
					huh.NewOption("Fortinet", "fortinet"),
					huh.NewOption("GlobalProtect", "gp"),
					huh.NewOption("AnyConnect", "anyconnect"),
					huh.NewOption("Juniper", "nc"),
					huh.NewOption("Pulse", "pulse"),
					huh.NewOption("F5", "f5"),
					huh.NewOption("Array", "array"),
				).
				Value(&data.Protocol),

			huh.NewInput().
				Title("Host").
				Prompt("> ").
				Value(&data.Host).
				Validate(func(s string) error {
					if s == "" {
						return errRequired
					}
					return nil
				}),

			huh.NewInput().
				Title("Username").
				Prompt("> ").
				Value(&data.Username),

			huh.NewInput().
				Title("Password").
				Prompt("> ").
				EchoMode(huh.EchoModePassword).
				Value(&data.Password).
				Description("Leave empty to keep existing or prompt"),

			huh.NewInput().
				Title("Flags").
				Prompt("> ").
				Value(&data.Flags).
				Description("Additional openconnect flags"),
		).Title(title).Description(" "),
	).WithShowHelp(true).WithTheme(formTheme()).WithWidth(width)
}

type SettingsFormData struct {
	DNS             string
	Reconnect       bool
	WifiInterface   string
	NetInterface    string
	TunnelInterface string
}

func NewSettingsFormData(settings *models.Settings) *SettingsFormData {
	return &SettingsFormData{
		DNS:             settings.GetDNS(),
		Reconnect:       settings.Reconnect,
		WifiInterface:   settings.GetWifiInterface(),
		NetInterface:    settings.GetNetInterface(),
		TunnelInterface: settings.GetTunnelInterface(),
	}
}

func (d *SettingsFormData) ToSettings() *models.Settings {
	return &models.Settings{
		DNS:             d.DNS,
		Reconnect:       d.Reconnect,
		WifiInterface:   d.WifiInterface,
		NetInterface:    d.NetInterface,
		TunnelInterface: d.TunnelInterface,
	}
}

func NewSettingsForm(data *SettingsFormData, width int) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("DNS Servers").
				Prompt("> ").
				Value(&data.DNS).
				Description("Space-separated DNS servers (e.g. 1.1.1.1 1.0.0.1)"),

			huh.NewConfirm().
				Title("Auto-reconnect").
				Value(&data.Reconnect).
				Description("Automatically reconnect on disconnect"),

			huh.NewInput().
				Title("WiFi Interface").
				Prompt("> ").
				Value(&data.WifiInterface).
				Description("For networksetup (default: Wi-Fi)"),

			huh.NewInput().
				Title("Network Interface").
				Prompt("> ").
				Value(&data.NetInterface).
				Description("For ifconfig (default: en0)"),

			huh.NewInput().
				Title("Tunnel Interface").
				Prompt("> ").
				Value(&data.TunnelInterface).
				Description("VPN tunnel interface (default: utun0)"),
		).Title("Settings").Description(" "),
	).WithShowHelp(true).WithTheme(formTheme()).WithWidth(width)
}

type DeleteConfirmData struct {
	Confirmed bool
}

func NewDeleteConfirmForm(name string, data *DeleteConfirmData, width int) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Delete \"" + name + "\"?").
				Value(&data.Confirmed),
		),
	).WithShowHelp(true).WithTheme(formTheme()).WithWidth(width)
}

type ExportFormData struct {
	Path      string
	StripANSI bool
}

func NewExportFormData() *ExportFormData {
	return &ExportFormData{
		Path:      DefaultExportPath(),
		StripANSI: true,
	}
}

func NewExportLogsForm(data *ExportFormData, width int) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Export Path").
				Prompt("> ").
				Value(&data.Path).
				Description("Path to save the log file"),
			huh.NewConfirm().
				Title("Strip ANSI codes").
				Value(&data.StripANSI),
		).Title("Export Logs").Description(" "),
	).WithShowHelp(true).WithTheme(formTheme()).WithWidth(width)
}

package models

type Config struct {
	Connections []Connection `json:"connections"`
	Settings    Settings     `json:"settings"`
}

func NewConfig() *Config {
	return &Config{
		Connections: []Connection{},
		Settings: Settings{
			DNS:       "",
			Reconnect: false,
		},
	}
}

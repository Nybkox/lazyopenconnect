package models

type ConnStatus int

const (
	StatusDisconnected ConnStatus = iota
	StatusConnecting
	StatusPrompting
	StatusConnected
	StatusExternal
	StatusReconnecting
	StatusQuitting
)

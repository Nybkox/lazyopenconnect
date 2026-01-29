package helpers

import "github.com/zalando/go-keyring"

const serviceName = "openconnect-tui"

func GetPassword(connectionID string) (string, error) {
	return keyring.Get(serviceName, connectionID)
}

func SetPassword(connectionID, password string) error {
	return keyring.Set(serviceName, connectionID, password)
}

func DeletePassword(connectionID string) error {
	return keyring.Delete(serviceName, connectionID)
}

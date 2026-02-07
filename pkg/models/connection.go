package models

type Connection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Host        string `json:"host"`
	Username    string `json:"username"`
	HasPassword bool   `json:"hasPassword"`
	ServerCert  string `json:"serverCert,omitempty"`
	Flags       string `json:"flags"`
}

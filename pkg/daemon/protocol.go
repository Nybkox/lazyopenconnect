package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"

	"github.com/Nybkox/lazyopenconnect/pkg/models"
)

type HelloCmd struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

type HelloResponse struct {
	Type       string `json:"type"`
	Version    string `json:"version"`
	Compatible bool   `json:"compatible"`
}

type GetStateCmd struct {
	Type string `json:"type"`
}

type ConnectCmd struct {
	Type     string `json:"type"`
	ConnID   string `json:"conn_id"`
	Password string `json:"password,omitempty"`
}

type DisconnectCmd struct {
	Type string `json:"type"`
}

type InputCmd struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ConfigUpdateCmd struct {
	Type   string        `json:"type"`
	Config models.Config `json:"config"`
}

type ShutdownCmd struct {
	Type string `json:"type"`
}

type GetLogsCmd struct {
	Type string `json:"type"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

type ClearLogsCmd struct {
	Type string `json:"type"`
}

type StateMsg struct {
	Type          string `json:"type"`
	Status        int    `json:"status"`
	ActiveConnID  string `json:"active_conn_id"`
	IP            string `json:"ip"`
	PID           int    `json:"pid"`
	TotalLogLines int    `json:"total_log_lines"`
	ExternalHost  string `json:"external_host,omitempty"`
}

type LogMsg struct {
	Type       string `json:"type"`
	Line       string `json:"line"`
	LineNumber int    `json:"line_number"`
}

type LogRangeMsg struct {
	Type       string   `json:"type"`
	From       int      `json:"from"`
	Lines      []string `json:"lines"`
	TotalLines int      `json:"total_lines"`
}

type PromptMsg struct {
	Type       string `json:"type"`
	IsPassword bool   `json:"is_password"`
}

type ConnectedMsg struct {
	Type string `json:"type"`
	IP   string `json:"ip"`
	PID  int    `json:"pid"`
}

type DisconnectedMsg struct {
	Type string `json:"type"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type KickedMsg struct {
	Type string `json:"type"`
}

type ReconnectingMsg struct {
	Type    string `json:"type"`
	ConnID  string `json:"conn_id"`
	Reason  string `json:"reason"`
	Attempt int    `json:"attempt"`
	Max     int    `json:"max"`
}

type CleanupCmd struct {
	Type string `json:"type"`
}

type CleanupStepMsg struct {
	Type string `json:"type"`
	Line string `json:"line"`
}

type CleanupDoneMsg struct {
	Type string `json:"type"`
}

type IncomingMsg struct {
	Type string
	raw  json.RawMessage
}

type messageEnvelope struct {
	Type string `json:"type"`
}

func WriteMsg(conn net.Conn, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = conn.Write(data)
	return err
}

func ReadMsg(r *bufio.Reader) (IncomingMsg, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return IncomingMsg{}, err
	}
	var env messageEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		return IncomingMsg{}, err
	}
	if env.Type == "" {
		return IncomingMsg{}, errors.New("missing message type")
	}
	return IncomingMsg{Type: env.Type, raw: json.RawMessage(line)}, nil
}

func (m IncomingMsg) Decode(dst any) error {
	if len(m.raw) == 0 {
		return errors.New("empty message")
	}
	return json.Unmarshal(m.raw, dst)
}

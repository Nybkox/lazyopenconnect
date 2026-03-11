package daemon

import (
	"testing"

	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
)

func TestIsPrompt(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "password prompt", line: "Password:", want: true},
		{name: "otp prompt", line: "Enter OTP?", want: true},
		{name: "plain log line", line: "Established DTLS connection", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrompt(tt.line); got != tt.want {
				t.Fatalf("isPrompt(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsPasswordPrompt(t *testing.T) {
	if !isPasswordPrompt("Enter password:") {
		t.Fatal("expected password prompt to be detected")
	}
	if isPasswordPrompt("Enter username:") {
		t.Fatal("did not expect username prompt to be treated as password prompt")
	}
}

func TestCheckLineForEventsMarksConnected(t *testing.T) {
	d := newTestDaemon()
	d.state.Status = StatusConnecting
	client := attachTestClient(t, d)

	done := make(chan struct{})
	go func() {
		d.checkLineForEvents("Configured as 10.10.10.5 with pid 4321")
		close(done)
	}()

	msg := readTestMsg(t, client)
	var connected ConnectedMsg
	if err := msg.Decode(&connected); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if connected.Type != "connected" {
		t.Fatalf("Type = %q, want %q", connected.Type, "connected")
	}
	if connected.IP != "10.10.10.5" {
		t.Fatalf("IP = %q, want %q", connected.IP, "10.10.10.5")
	}
	if connected.PID != 4321 {
		t.Fatalf("PID = %d, want %d", connected.PID, 4321)
	}
	<-done

	d.stateMu.RLock()
	status := d.state.Status
	ip := d.state.IP
	pid := d.state.PID
	d.stateMu.RUnlock()

	if status != StatusConnected {
		t.Fatalf("status = %v, want %v", status, StatusConnected)
	}
	if ip != "10.10.10.5" {
		t.Fatalf("IP = %q, want %q", ip, "10.10.10.5")
	}
	if pid != 4321 {
		t.Fatalf("PID = %d, want %d", pid, 4321)
	}
}

func TestCheckLineForEventsUpdatesTunnelInterface(t *testing.T) {
	d := newTestDaemon()
	d.state.NetworkSnapshot = &helpers.NetworkSnapshot{}

	d.checkLineForEvents("Using tun device utun9")

	d.stateMu.RLock()
	tunnelInterface := d.state.NetworkSnapshot.TunnelInterface
	d.stateMu.RUnlock()

	if tunnelInterface != "utun9" {
		t.Fatalf("TunnelInterface = %q, want %q", tunnelInterface, "utun9")
	}
}

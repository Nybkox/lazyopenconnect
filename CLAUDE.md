# CLAUDE.md - Agent Instructions for lazyopenconnect

## Project Overview

Go TUI application for managing OpenConnect VPN connections. Built with Bubble Tea (Elm architecture).

- **Module**: `github.com/Nybkox/lazyopenconnect`
- **Go Version**: 1.25.4
- **Requires**: Root access (`sudo`) to run

## Build & Run Commands

```bash
# Build
go build -o lazyopenconnect

# Run (requires root)
sudo ./lazyopenconnect

# Run directly
sudo go run main.go

# Format code
go fmt ./...
goimports -w .

# Vet/lint
go vet ./...

# Tidy dependencies
go mod tidy
```

## Testing

No tests exist yet. When adding tests:

```bash
# Run all tests
go test ./...

# Run single test
go test -run TestFunctionName ./pkg/app/

# Run tests in specific package
go test ./pkg/models/

# Verbose output
go test -v ./...

# With coverage
go test -cover ./...
```

## Project Structure

```
.
├── main.go                          # Entry point, routes to daemon or client
├── pkg/
│   ├── app/                         # TUI client (Bubble Tea application)
│   │   ├── app.go                   # App struct, Init(), View()
│   │   ├── state.go                 # Client-side view of daemon state
│   │   ├── update.go                # Update() message routing
│   │   ├── keys.go                  # KeyMap definitions
│   │   ├── messages.go              # Custom tea.Msg types
│   │   ├── forms.go                 # Form handling logic
│   │   ├── daemon_client.go         # Daemon connection client
│   │   ├── handlers_*.go            # Handlers by concern
│   ├── daemon/                      # Background daemon (NEW)
│   │   ├── daemon.go                # Core daemon, socket handling
│   │   ├── vpn.go                   # VPN process management (PTY, I/O)
│   │   └── protocol.go              # JSON message protocol
│   ├── models/                      # Data structures (shared)
│   │   ├── config.go                # Config struct
│   │   ├── connection.go            # Connection struct
│   │   ├── settings.go              # Settings struct
│   ├── controllers/helpers/         # Business logic
│   │   ├── vpn.go                   # Cleanup commands (legacy)
│   │   ├── config.go                # Config file I/O
│   │   ├── keychain.go              # Password storage
│   │   └── forms.go                 # Form data structs
│   └── presentation/                # UI rendering
│       ├── layout.go                # Render functions
│       └── styles.go                # Lipgloss styles
```

## Code Style Guidelines

### Imports

Order: stdlib, blank line, external, blank line, internal. Use aliases for common packages:

```go
import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/Nybkox/lazyopenconnect/pkg/models"
)
```

### Types

Prefer `type` over `interface` for type definitions. Use `iota` for enums:

```go
type ConnStatus int

const (
    StatusDisconnected ConnStatus = iota
    StatusConnecting
    StatusConnected
)
```

Use JSON tags for serialized structs:

```go
type Connection struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Host     string `json:"host"`
}
```

### Naming Conventions

- **Exported**: `PascalCase` (e.g., `App`, `NewState`, `HandleVPNLog`)
- **Unexported**: `camelCase` (e.g., `configDir`, `spinnerFrame`)
- **Constants**: `PascalCase` for exported, `camelCase` for internal
- **Acronyms**: Keep uppercase (e.g., `VPN`, `DNS`, `IP`, `PID`)
- **Receivers**: Single letter matching type (e.g., `(a *App)`, `(s *State)`)

### Error Handling

Return errors directly. Use `_` for intentionally ignored errors:

```go
// Return errors
func LoadConfig() (*models.Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return models.NewConfig(), nil
        }
        return nil, err
    }
    // ...
}

// Ignore when appropriate (e.g., best-effort cleanup)
_ = helpers.SaveConfig(a.State.Config)
```

### Control Flow

Prefer early returns over nested code:

```go
// Good
func (a *App) connect() (tea.Model, tea.Cmd) {
    conn := a.State.SelectedConnection()
    if conn == nil {
        return a, nil
    }
    if a.State.Status != StatusDisconnected {
        return a, nil
    }
    // main logic...
}

// Avoid deep nesting
```

### Bubble Tea Patterns

Message handlers return `(tea.Model, tea.Cmd)`:

```go
func (a *App) handleVPNLog(msg helpers.VPNLogMsg) (tea.Model, tea.Cmd) {
    a.State.OutputLines = append(a.State.OutputLines, string(msg))
    a.viewport.SetContent(a.renderOutput())
    return a, helpers.WaitForLog()
}
```

Use `tea.Batch()` for multiple commands:

```go
return a, tea.Batch(
    helpers.StartVPN(conn, password),
    spinnerTick(),
)
```

### Comments

- File-level comments for package purpose
- Function comments for exported functions
- Inline comments sparingly, only when non-obvious

```go
// Keys definitions for the TUI application.
package app

// GetPassword retrieves a password from the system keychain
func GetPassword(connectionID string) (string, error) {
```

## Key Dependencies

- **bubbletea**: TUI framework (Elm architecture)
- **bubbles**: TUI components (viewport, textinput)
- **lipgloss**: Terminal styling
- **huh**: Form handling
- **go-keyring**: System keychain access
- **creack/pty**: PTY for VPN process (now in daemon)

## Architecture Notes

### Client-Daemon Model

The application uses a client-daemon architecture:

**Daemon Lifecycle:**
- Client auto-detects running daemon on startup (via socket file)
- Spawns daemon automatically if not running
- **Version mismatch handling**: If client and daemon versions differ, daemon auto-shuts down (see `handleHello()`) so client can spawn a matching version
- Daemon survives client disconnects; use `lazyopenconnect daemon stop` to kill it

**Logging:**
- Daemon logs to `~/.config/lazyopenconnect/daemon.log` (internal daemon events)
- VPN output logs to `~/.config/lazyopenconnect/vpn.log` (connection output)
- Both use append mode; no automatic rotation

1. **Daemon** (`pkg/daemon/`) runs as a background process, owns the VPN connection
2. **Client** (`pkg/app/`) is the TUI that connects to the daemon via Unix socket
3. **Protocol** - JSON messages over Unix domain socket (`~/.config/lazyopenconnect/daemon.sock`)

**Message flow:**
```
TUI Client → Daemon → openconnect process
     ↑         ↓
   updates   events (logs, prompts, state changes)
```

### State Management

The **daemon** is the source of truth for connection state. The client maintains a cached view:

```go
// Daemon state (pkg/daemon/daemon.go)
d.state.Status           // Disconnected, Connecting, Prompting, Connected
d.state.ActiveConnID     // Currently active connection
d.state.IP              // Assigned VPN IP
d.state.PID             // openconnect process ID

// Client state (pkg/app/state.go) - mirrors daemon
a.State.Status           // Synced from daemon 'state' messages
a.State.OutputLines      // Log buffer from daemon
```

### Log Handling (Paginated/Lazy Loading)

VPN logs are stored in `~/.config/lazyopenconnect/vpn.log` (not in daemon memory). The TUI fetches log ranges on-demand:

```go
// Client requests log range
GetLogsCmd{Type: "get_logs", From: 0, To: 1000}

// Daemon responds with lines
LogRangeMsg{Type: "log_range", From: 0, Lines: [...], TotalLines: 5000}
```

**Windowing strategy:**
- `MaxLoadedLines = 1000` - Max lines held in TUI memory at once
- As user scrolls, `shouldFetchLogs()` checks if viewport exceeds loaded range
- Fetches new centered window around current scroll position
- Prevents memory bloat on long-running connections

### Daemon Protocol

JSON messages over Unix socket (`pkg/daemon/protocol.go`):

**Client → Daemon:**
```go
HelloCmd{Type: "hello", Version: "..."}
ConnectCmd{Type: "connect", ConnID: "...", Password: "..."}
DisconnectCmd{Type: "disconnect"}
InputCmd{Type: "input", Value: "..."}
ConfigUpdateCmd{Type: "config_update", Config: {...}}
GetStateCmd{Type: "get_state"}
ShutdownCmd{Type: "shutdown"}
```

**Daemon → Client:**
```go
HelloResponse{Type: "hello_response", Version: "...", Compatible: true}
StateMsg{Type: "state", Status: ..., ActiveConnID: ..., IP: ..., PID: ...}
LogMsg{Type: "log", Line: "..."}
PromptMsg{Type: "prompt", IsPassword: true}
ConnectedMsg{Type: "connected", IP: "...", PID: ...}
DisconnectedMsg{Type: "disconnected"}
ErrorMsg{Type: "error", Code: "...", Message: "..."}
KickedMsg{Type: "kicked"}  // Another client connected
```

### Presentation Layer

UI rendering in `pkg/presentation/` is separate from logic. `Render()` takes state and returns string:

```go
type RenderFunc func(state *State, spinnerFrame int) string
```

## Common Tasks

### Adding a new connection field

1. Add to `pkg/models/connection.go`
2. Add to `ConnectionFormData` in `pkg/controllers/helpers/forms.go`
3. Update `ToConnection()` and `NewConnectionFormData()`
4. Add form field in `NewConnectionForm()`

### Adding a new message type

1. Define message struct in `pkg/daemon/protocol.go` (if daemon-related)
2. Add handler in `pkg/daemon/daemon.go` `handleMessage()` switch
3. Add handler in `pkg/app/update.go` `handleDaemonMsg()` switch
4. Create handler method `handle*()` in appropriate file

### Adding a new pane

1. Add to `FocusedPane` enum in `pkg/app/state.go`
2. Add key binding in `pkg/app/keys.go`
3. Add render function in `pkg/presentation/layout.go`
4. Add update handler in `pkg/app/handlers_pane.go`

## RULES

- Do not write comments if not absolutely necessary

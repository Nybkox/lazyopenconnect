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
├── main.go                          # Entry point, initializes App
├── pkg/
│   ├── app/                         # Bubble Tea application
│   │   ├── app.go                   # App struct, Init(), View()
│   │   ├── state.go                 # State struct, status enums
│   │   ├── update.go                # Update() message routing
│   │   ├── keys.go                  # KeyMap definitions
│   │   ├── messages.go              # Custom tea.Msg types
│   │   ├── forms.go                 # Form handling logic
│   │   ├── handlers_*.go            # Handlers by concern
│   ├── models/                      # Data structures
│   │   ├── config.go                # Config struct
│   │   ├── connection.go            # Connection struct
│   │   ├── settings.go              # Settings struct
│   ├── controllers/helpers/         # Business logic
│   │   ├── vpn.go                   # VPN process management
│   │   ├── config.go                # Config file I/O
│   │   ├── keychain.go              # Password storage
│   │   ├── forms.go                 # Form data structs
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
- **creack/pty**: PTY for VPN process

## Architecture Notes

### State Management

All state in `pkg/app/state.go`. Access via `a.State.*`:

```go
a.State.Status           // Connection status enum
a.State.Config           // Loaded config with connections
a.State.FocusedPane      // Current UI focus
a.State.OutputLines      // VPN output log
```

### Async Communication

VPN output streams via channels in `pkg/controllers/helpers/vpn.go`:

```go
var LogChan = make(chan string, 100)
var PromptChan = make(chan VPNPromptMsg, 10)
```

Consumed via commands like `helpers.WaitForLog()`.

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

1. Define in `pkg/controllers/helpers/vpn.go` or `pkg/app/messages.go`
2. Add case in `pkg/app/update.go` `Update()` switch
3. Create handler method `handle*()` 

### Adding a new pane

1. Add to `FocusedPane` enum in `pkg/app/state.go`
2. Add key binding in `pkg/app/keys.go`
3. Add render function in `pkg/presentation/layout.go`
4. Add update handler in `pkg/app/handlers_pane.go`

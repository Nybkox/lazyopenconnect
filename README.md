# lazyopenconnect

A TUI for managing OpenConnect VPN connections. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![License](https://img.shields.io/badge/license-MIT-green)
![Go Version](https://img.shields.io/badge/go-1.25.4-blue)
![Linux](https://img.shields.io/badge/Linux-FCC624?logo=linux&logoColor=black)
![macOS](https://img.shields.io/badge/macOS-000000?logo=apple&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-not%20supported-red?logo=windows&logoColor=white)

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/Nybkox/lazyopenconnect/master/install.sh | bash
```

## Why?

FortiClient sucks. Sometimes it won't connect. Sometimes it won't reconnect. And the free version? Refuses to remember your password and auto-reconnect on failures...

Using the OpenConnect CLI directly works, but gets annoying fast. I stuck with a janky bash script for a while, but eventually decided this problem deserved a proper solution — a [lazygit](https://github.com/jesseduffield/lazygit)-style TUI that just works.

## Features

- **Connection management** - Create, edit, delete VPN profiles
- **Multi-pane interface** - Status, connections, settings, output log, and input in one view
- **Secure password storage** - Passwords stored in system keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager)
- **Auto-reconnect** - Automatically reconnect when connection drops (configurable)
- **External VPN detection** - Detects and displays OpenConnect processes started outside the TUI
- **Detach/attach support** - Close the TUI while keeping VPN connected (`q` to detach, `Q` to quit)
- **Daemon architecture** - VPN runs in a background daemon, TUI connects via Unix socket
- **Automatic daemon management** - Daemon auto-restarts on version mismatch, auto-starts with client, and supports `daemon stop all` for stale processes
- **Efficient log handling** - VPN logs stored in file with lazy loading (paginated fetch as you scroll)
- **Fast log reset** - Clear VPN logs with `x` then `x` in Output pane (clears both UI window and `vpn.log`)
- **Interactive prompts** - Handle 2FA, OTP, and other authentication prompts directly in the TUI
- **Smart disconnect cleanup** - Uses OpenConnect built-in cleanup first, then falls back to manual route/DNS/interface cleanup
- **Reconnect on wake** - Better reliability after laptop sleep/wake cycles
- **Config-to-daemon sync** - Connection create/edit changes are synced to daemon immediately
- **Hardened daemon locking** - Reduced stale lock and duplicate daemon edge cases
- **Vim-style navigation** - `j/k` for movement, `g/G` for top/bottom, `ctrl+d/u` for page scroll

## Screenshots

![lazyopenconnect ui](screenshot.png)

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Nybkox/lazyopenconnect/master/install.sh | bash
```

Installs to `~/.local/bin` (no sudo required). Creates `lzcon` alias. Prompts to add to PATH if needed.

The installer also handles piped execution safely and uses OS-specific install directories when needed.

```bash
# Install system-wide (requires sudo)
sudo curl -fsSL https://raw.githubusercontent.com/Nybkox/lazyopenconnect/master/install.sh | bash -s -- --system
```

### Homebrew

```bash
brew tap nybkox/tap
brew install lazyopenconnect
```

### Download Binary

Download the latest release from [GitHub Releases](https://github.com/Nybkox/lazyopenconnect/releases):

| Platform | Architecture  | Download                                |
| -------- | ------------- | --------------------------------------- |
| macOS    | Intel         | `lazyopenconnect_*_darwin_amd64.tar.gz` |
| macOS    | Apple Silicon | `lazyopenconnect_*_darwin_arm64.tar.gz` |
| Linux    | x86_64        | `lazyopenconnect_*_linux_amd64.tar.gz`  |
| Linux    | ARM64         | `lazyopenconnect_*_linux_arm64.tar.gz`  |

```bash
# Extract and install
tar -xzf lazyopenconnect_*.tar.gz
mkdir -p ~/.local/bin
mv lazyopenconnect ~/.local/bin/
```

### From Source

```bash
git clone https://github.com/Nybkox/lazyopenconnect.git
cd lazyopenconnect
go build -o lazyopenconnect
mkdir -p ~/.local/bin
mv lazyopenconnect ~/.local/bin/
```

### Requirements

- **OpenConnect** - Must be installed and accessible in PATH
- **Root access** - Required for VPN operations
- **macOS** - Currently optimized for macOS (network cleanup commands)

Install OpenConnect:

```bash
# macOS
brew install openconnect

# Ubuntu/Debian
sudo apt install openconnect

# Fedora
sudo dnf install openconnect
```

## Usage

```bash
# Run (requires root)
sudo lazyopenconnect

# Or use the short alias
sudo lzcon

# Daemon commands
lazyopenconnect daemon status  # Check if daemon is running
lazyopenconnect daemon start   # Start daemon manually
lazyopenconnect daemon stop    # Stop daemon and disconnect VPN
lazyopenconnect daemon stop all # Stop all matching stale daemons
```

## Uninstall

```bash
# Uninstall (prompts for config removal)
lazyopenconnect uninstall

# Uninstall and remove all data
lazyopenconnect uninstall --purge

# Uninstall binary only, keep config and passwords
lazyopenconnect uninstall --keep-config

# If installed to /usr/local/bin (requires sudo)
sudo lazyopenconnect uninstall

# Homebrew
brew uninstall lazyopenconnect
```

## Configuration

Configuration is stored in `~/.config/lazyopenconnect/config.json`.

### Connection Options

| Field         | Description                                                           |
| ------------- | --------------------------------------------------------------------- |
| `name`        | Display name for the connection                                       |
| `protocol`    | VPN protocol: `gp` (GlobalProtect), `anyconnect`, `nc`, `pulse`, etc. |
| `host`        | VPN server hostname                                                   |
| `username`    | Login username (optional)                                             |
| `hasPassword` | Whether password is stored in keychain                                |
| `flags`       | Additional openconnect flags (e.g., `--servercert=...`)               |

### Settings

| Setting           | Description                             | Default           |
| ----------------- | --------------------------------------- | ----------------- |
| `dns`             | DNS servers to restore after disconnect | `1.1.1.1 1.0.0.1` |
| `reconnect`       | Auto-reconnect on connection drop       | `false`           |
| `autoCleanup`     | Run cleanup automatically on disconnect | `true`            |
| `wifiInterface`   | Wi-Fi interface name (for DNS restore)  | `Wi-Fi`           |
| `netInterface`    | Network interface name                  | `en0`             |
| `tunnelInterface` | VPN tunnel interface                    | `utun0`           |

## Supported Protocols

All protocols supported by OpenConnect:

- `anyconnect` - Cisco AnyConnect
- `gp` - Palo Alto GlobalProtect
- `nc` - Juniper Network Connect
- `pulse` - Pulse Secure
- `f5` - F5 BIG-IP
- `fortinet` - Fortinet FortiGate
- `array` - Array Networks

## Architecture

Follows a **Client-Daemon** architecture built on Bubble Tea's Elm-style pattern:

**1. Daemon (`pkg/daemon/`)** - Background process that manages the VPN connection lifecycle. Runs continuously even when the TUI is closed. Handles PTY I/O, prompt detection, connection state, and network cleanup. Communicates with clients via Unix domain socket using a JSON protocol.

**2. App (`pkg/app/`)** - TUI client implementing Bubble Tea's `Model` interface. Connects to the daemon on startup, sends commands (connect, disconnect, input), and displays state updates. Multiple clients can connect; the last one takes control.

**3. State (`pkg/app/state.go`)** - Client-side view of daemon state. Connection status, pane focus, form state, output buffer. The daemon is the source of truth; the client syncs via messages.

**4. Helpers (`pkg/controllers/helpers/`)** - Business logic decoupled from UI. `config.go` persists JSON. `keychain.go` wraps system credential storage. Cleanup commands for network interface restoration.

**5. Models (`pkg/models/`)** - Pure data structs with JSON tags. `Connection` (profile), `Settings` (preferences), `Config` (root). Shared between client and daemon.

**6. Presentation (`pkg/presentation/`)** - Pure render functions: State in → styled string out. Multi-pane layout, scrollbars, form overlays. No business logic.

## Tech Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework (Elm architecture)
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Huh](https://github.com/charmbracelet/huh) - Form components
- [go-keyring](https://github.com/zalando/go-keyring) - Cross-platform keychain
- [creack/pty](https://github.com/creack/pty) - PTY for process I/O

## Key Bindings

| Key                 | Action                                             |
| ------------------- | -------------------------------------------------- |
| `q`                 | **Detach** - Close TUI, keep VPN running           |
| `Q` / `Ctrl+C`      | **Quit** - Disconnect VPN and exit                 |
| `Enter`             | Connect to selected connection                     |
| `d`                 | Disconnect current connection                      |
| `c`                 | Run network cleanup                                |
| `n`                 | Add new connection                                 |
| `e`                 | Edit selected connection                           |
| `x`                 | Delete connection                                  |
| `1-4`               | Focus pane (status, connections, settings, output) |
| `Tab` / `Shift+Tab` | Cycle focus                                        |
| `j/k` or `↑/↓`      | Navigate                                           |
| `g/G`               | Top/bottom                                         |
| `Ctrl+d/u`          | Page scroll                                        |
| `x` then `x` (Output pane) | Clear VPN logs (double-tap confirm)         |

## Troubleshooting

### "Requires root" error

OpenConnect needs root privileges to create network interfaces. Always run with `sudo`.

### "Daemon version mismatch" error

The client and daemon must run the same version. Stop the daemon to let the client spawn a new one:

```bash
lazyopenconnect daemon stop
sudo lazyopenconnect
```

If you suspect stale background daemons from older binaries or aliases, run:

```bash
lazyopenconnect daemon stop all
```

### Debugging with daemon logs

The daemon writes logs to `~/.config/lazyopenconnect/daemon.log`. Useful for debugging connection issues:

```bash
# Follow logs in real-time
tail -f ~/.config/lazyopenconnect/daemon.log

# View recent logs
cat ~/.config/lazyopenconnect/daemon.log
```

### VPN log file

VPN connection output is stored in `~/.config/lazyopenconnect/vpn.log` (not in memory). The TUI uses lazy loading to fetch log ranges as you scroll, keeping memory usage constant even for long-running connections:

In the Output pane, press `x` then `x` within 2 seconds to clear logs. This clears both the visible output window and the underlying `vpn.log` file via the daemon.

```bash
# View full VPN session log
cat ~/.config/lazyopenconnect/vpn.log
```

### Network issues after disconnect

Press `c` in the Connections pane to run cleanup, which:

1. Brings down the tunnel interface
2. Flushes routing table
3. Restarts network interface
4. Restores DNS settings
5. Flushes DNS cache

### Password not being sent

Ensure "Save password" is enabled when creating/editing the connection. Passwords are stored securely in your system keychain.

### Connection drops immediately

Check the Output pane for error messages. Common issues:

- Server certificate not trusted - add `--servercert=<hash>` to flags
- Missing dependencies - ensure OpenConnect is installed
- Wrong protocol - try different protocol options

## License

MIT

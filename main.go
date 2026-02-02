package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"

	"github.com/Nybkox/lazyopenconnect/pkg/app"
	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/daemon"
	"github.com/Nybkox/lazyopenconnect/pkg/presentation"
	"github.com/Nybkox/lazyopenconnect/pkg/version"
)

var (
	buildVersion = "dev"
	commit       = "none"
	date         = "unknown"

	debug       bool
	showHelp    bool
	showVersion bool
)

func init() {
	pflag.BoolVar(&debug, "debug", false, "Enable debug logging in daemon")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help")
	pflag.BoolVarP(&showVersion, "version", "v", false, "Show version")
	pflag.Parse()
}

func main() {
	version.Current = buildVersion

	if showHelp {
		printHelp()
		return
	}

	if showVersion {
		fmt.Printf("lazyopenconnect %s (commit: %s, built: %s)\n", buildVersion, commit, date)
		return
	}

	args := pflag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "daemon":
			if len(args) > 1 && args[1] == "run" {
				if err := daemon.Run(debug); err != nil {
					fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
					os.Exit(1)
				}
				return
			}
			handleDaemonCmd(args[1:])
			return
		case "update":
			handleUpdate()
			return
		case "uninstall":
			handleUninstall()
			return
		}
	}

	if os.Geteuid() != 0 {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Requires root. Run: sudo lazyopenconnect")
			os.Exit(1)
		}
		sudoPath, err := exec.LookPath("sudo")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Requires root. Run: sudo lazyopenconnect")
			os.Exit(1)
		}
		fmt.Println("lazyopenconnect requires sudo to run openconnect...")
		args := append([]string{"sudo", exe}, os.Args[1:]...)
		if err := syscall.Exec(sudoPath, args, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "Requires root. Run: sudo lazyopenconnect")
			os.Exit(1)
		}
	}

	socketPath, err := daemon.SocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get socket path: %v\n", err)
		os.Exit(1)
	}

	var conn net.Conn
	var reader *bufio.Reader

	for attempt := 0; attempt < 2; attempt++ {
		if !daemonRunning(socketPath) {
			if err := spawnDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
				os.Exit(1)
			}
			time.Sleep(200 * time.Millisecond)
		}

		conn, err = net.Dial("unix", socketPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
			os.Exit(1)
		}

		daemon.WriteMsg(conn, daemon.HelloCmd{
			Type:    "hello",
			Version: version.Current,
		})

		reader = bufio.NewReader(conn)
		resp, err := daemon.ReadMsg(reader)
		if err != nil {
			conn.Close()
			fmt.Fprintf(os.Stderr, "Failed to read daemon response: %v\n", err)
			os.Exit(1)
		}

		if compatible, ok := resp["compatible"].(bool); ok && compatible {
			break
		}

		daemonVersion, _ := resp["version"].(string)
		conn.Close()

		if attempt == 0 {
			fmt.Fprintf(os.Stderr, "Daemon version mismatch (client: %s, daemon: %s), restarting...\n",
				version.Current, daemonVersion)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		fmt.Fprintln(os.Stderr, "Failed to restart daemon with matching version")
		os.Exit(1)
	}
	defer conn.Close()

	cfg, err := helpers.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	daemon.WriteMsg(conn, daemon.ConfigUpdateCmd{
		Type:   "config_update",
		Config: *cfg,
	})

	daemon.WriteMsg(conn, daemon.GetStateCmd{Type: "get_state"})

	a := app.New(cfg)
	a.RenderView = presentation.Render
	a.ConnectToDaemon(conn)

	p := tea.NewProgram(a, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func daemonRunning(socketPath string) bool {
	_, err := os.Stat(socketPath)
	return err == nil
}

func spawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"daemon", "run"}
	if debug {
		args = append(args, "--debug")
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Process.Release()
}

func printHelp() {
	fmt.Println(`lazyopenconnect - TUI for managing OpenConnect VPN connections

Usage:
  lazyopenconnect [flags]
  lazyopenconnect [command]

Commands:
  daemon start    Start the background daemon
  daemon stop     Stop the background daemon
  daemon status   Check if daemon is running
  update          Check for and install updates
  uninstall       Remove lazyopenconnect

Flags:`)
	pflag.PrintDefaults()
	fmt.Println("\nRequires sudo to run openconnect.")
}

func handleDaemonCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: lazyopenconnect daemon <start|stop|status>")
		return
	}

	socketPath, err := daemon.SocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get socket path: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "start":
		if daemonRunning(socketPath) {
			fmt.Println("Daemon is already running")
			return
		}
		if err := spawnDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(200 * time.Millisecond)
		if daemonRunning(socketPath) {
			fmt.Println("Daemon started")
		} else {
			fmt.Fprintln(os.Stderr, "Daemon failed to start")
			os.Exit(1)
		}

	case "stop":
		if !daemonRunning(socketPath) {
			fmt.Println("Daemon is not running")
			return
		}
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
			os.Exit(1)
		}
		daemon.WriteMsg(conn, daemon.ShutdownCmd{Type: "shutdown"})
		conn.Close()
		fmt.Println("Daemon stopped")

	case "status":
		if daemonRunning(socketPath) {
			fmt.Println("Daemon is running")
		} else {
			fmt.Println("Daemon is not running")
		}

	default:
		fmt.Println("Usage: lazyopenconnect daemon <start|stop|status>")
	}
}

func handleUpdate() {
	if helpers.IsHomebrewInstall() {
		fmt.Println("Installed via Homebrew.")
		fmt.Println("Update with: brew upgrade lazyopenconnect")
		return
	}

	fmt.Println("Checking for updates...")

	info, err := helpers.CheckForUpdate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	if !info.Available {
		fmt.Printf("Already up to date (v%s)\n", info.Current)
		return
	}

	fmt.Printf("\nUpdate available: v%s -> v%s\n", info.Current, info.Latest)
	fmt.Print("Do you want to update? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Update cancelled")
		return
	}

	fmt.Println("Downloading update...")
	if err := helpers.PerformUpdate(); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Update successful! Restart lazyopenconnect to use the new version.")
}

func handleUninstall() {
	var purge, keepConfig bool

	for _, arg := range os.Args[2:] {
		switch arg {
		case "--purge":
			purge = true
		case "--keep-config":
			keepConfig = true
		case "--help", "-h":
			fmt.Println("Usage: lazyopenconnect uninstall [OPTIONS]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --purge        Remove config, keychain entries, and PATH export without prompting")
			fmt.Println("  --keep-config  Only remove binary, keep config and keychain")
			fmt.Println("  --help         Show this help message")
			return
		}
	}

	if purge && keepConfig {
		fmt.Fprintln(os.Stderr, "Error: --purge and --keep-config are mutually exclusive")
		os.Exit(1)
	}

	if helpers.IsHomebrewInstall() {
		fmt.Println("Installed via Homebrew.")
		fmt.Println("Uninstall with: brew uninstall lazyopenconnect")
		return
	}

	binaryPath := helpers.GetInstallPath()
	if binaryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: No installation found")
		os.Exit(1)
	}

	if !purge {
		fmt.Printf("Uninstall lazyopenconnect from %s? [y/N]: ", binaryPath)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Uninstall cancelled")
			return
		}
	}

	if err := helpers.RemoveBinary(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	_ = helpers.RemoveAlias()

	if keepConfig {
		fmt.Println("Uninstall complete (config preserved)")
		return
	}

	removeData := purge
	if !purge {
		fmt.Print("Remove config, keychain entries, and PATH export? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		removeData = response == "y" || response == "yes"
	}

	if removeData {
		_ = helpers.RemoveKeychain()
		_ = helpers.RemoveConfigDir()
		_ = helpers.RemovePathExport()
	}

	fmt.Println("Uninstall complete")
}

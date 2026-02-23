package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
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

type daemonConnStatus int

const (
	daemonNotRunning daemonConnStatus = iota
	daemonReachable
	daemonInaccessible
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

	socketPath, err := daemon.SocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get socket path: %v\n", err)
		os.Exit(1)
	}

	if buildVersion == "dev" && daemonStatus(socketPath) == daemonReachable {
		fmt.Println("Dev mode: restarting daemon...")
		shutdownDaemon(socketPath)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if daemonStatus(socketPath) != daemonReachable {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	var conn net.Conn
	var reader *bufio.Reader

	for attempt := 0; attempt < 2; attempt++ {
		status := daemonStatus(socketPath)
		switch status {
		case daemonInaccessible:
			fmt.Println("Daemon socket is not accessible, restarting with sudo...")
			if err := stopAndRespawnDaemon(socketPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to restart daemon: %v\n", err)
				os.Exit(1)
			}
			ready := waitForDaemonReady(socketPath, 3*time.Second)
			if ready != daemonReachable {
				printInaccessibleDaemonHint()
				os.Exit(1)
			}
		case daemonNotRunning:
			if err := spawnDaemon(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
				os.Exit(1)
			}
			ready := waitForDaemonReady(socketPath, 2*time.Second)
			if ready != daemonReachable {
				if ready == daemonInaccessible {
					printInaccessibleDaemonHint()
				} else {
					fmt.Fprintln(os.Stderr, "Failed to start daemon: daemon did not become ready")
				}
				os.Exit(1)
			}
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

func daemonStatus(socketPath string) daemonConnStatus {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		if isSocketPermissionErr(err) {
			if _, statErr := os.Stat(socketPath); statErr == nil {
				return daemonInaccessible
			}
		}
		return daemonNotRunning
	}
	conn.Close()
	return daemonReachable
}

func waitForDaemonReady(socketPath string, timeout time.Duration) daemonConnStatus {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := daemonStatus(socketPath)
		if status == daemonReachable || status == daemonInaccessible {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	return daemonNotRunning
}

func isSocketPermissionErr(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}

func stopAndRespawnDaemon(socketPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	stopArgs := []string{exe, "daemon", "stop", "all"}
	stopCmd := exec.Command("sudo", stopArgs...)
	stopCmd.Stdin = os.Stdin
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		return err
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if daemonStatus(socketPath) == daemonNotRunning {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := spawnDaemon(); err != nil {
		return err
	}

	return nil
}

func printInaccessibleDaemonHint() {
	fmt.Fprintln(os.Stderr, "Daemon is running but not accessible for this user.")
	fmt.Fprintln(os.Stderr, "This usually means an older root-owned daemon socket.")
	fmt.Fprintln(os.Stderr, "Fix once with: sudo lazyopenconnect daemon stop all")
	fmt.Fprintln(os.Stderr, "Then run: lazyopenconnect")
}

func shutdownDaemon(socketPath string) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return
	}
	daemon.WriteMsg(conn, daemon.ShutdownCmd{Type: "shutdown"})
	conn.Close()
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

	if os.Geteuid() == 0 {
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

	fmt.Println("Starting privileged daemon (sudo required)...")
	sudoArgs := append([]string{"-b", exe}, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printHelp() {
	fmt.Println(`lazyopenconnect - TUI for managing OpenConnect VPN connections

Usage:
  lazyopenconnect [flags]
  lazyopenconnect [command]

Commands:
  daemon start    Start the background daemon
  daemon stop     Stop the background daemon
  daemon stop all Stop all matching stale daemons
  daemon status   Check if daemon is running
  update          Check for and install updates
  uninstall       Remove lazyopenconnect

Flags:`)
	pflag.PrintDefaults()
	fmt.Println("\nRequires sudo to run openconnect.")
}

func handleDaemonCmd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: lazyopenconnect daemon <start|stop all|status>")
		return
	}

	socketPath, err := daemon.SocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get socket path: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "start":
		status := daemonStatus(socketPath)
		if status == daemonReachable {
			fmt.Println("Daemon is already running")
			return
		}
		if status == daemonInaccessible {
			fmt.Fprintln(os.Stderr, "Daemon is already running but not accessible for this user")
			fmt.Fprintln(os.Stderr, "Run: sudo lazyopenconnect daemon stop all")
			os.Exit(1)
		}
		if err := spawnDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
			os.Exit(1)
		}
		ready := waitForDaemonReady(socketPath, 2*time.Second)
		if ready == daemonReachable {
			fmt.Println("Daemon started")
		} else {
			if ready == daemonInaccessible {
				fmt.Fprintln(os.Stderr, "Daemon started but socket is not accessible for this user")
				fmt.Fprintln(os.Stderr, "Run: sudo lazyopenconnect daemon stop all")
			} else {
				fmt.Fprintln(os.Stderr, "Daemon failed to start")
			}
			os.Exit(1)
		}

	case "stop":
		stopAll := len(args) > 1 && args[1] == "all"

		if os.Geteuid() != 0 && needsSudoForStop(socketPath, stopAll) {
			reExecWithSudo()
			return
		}

		if stopAll {
			if err := stopAllDaemons(socketPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to stop all daemons: %v\n", err)
				os.Exit(1)
			}
			return
		}

		status := daemonStatus(socketPath)
		if status == daemonNotRunning {
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
		status := daemonStatus(socketPath)
		if status == daemonReachable {
			fmt.Println("Daemon is running")
		} else if status == daemonInaccessible {
			fmt.Println("Daemon is running (inaccessible for current user)")
		} else {
			fmt.Println("Daemon is not running")
		}

	default:
		fmt.Println("Usage: lazyopenconnect daemon <start|stop all|status>")
	}
}

type daemonProc struct {
	pid  int
	comm string
	args []string
}

func needsSudoForStop(socketPath string, stopAll bool) bool {
	if stopAll {
		procs, err := findMatchingDaemonProcs()
		if err != nil {
			return false
		}
		for _, proc := range procs {
			p, err := os.FindProcess(proc.pid)
			if err != nil {
				continue
			}
			if err := p.Signal(syscall.Signal(0)); err != nil {
				if errors.Is(err, syscall.EPERM) {
					return true
				}
			}
		}
		status := daemonStatus(socketPath)
		return status == daemonInaccessible
	}
	return daemonStatus(socketPath) == daemonInaccessible
}

func reExecWithSudo() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot determine executable path")
		os.Exit(1)
	}
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "sudo not found; run manually with: sudo lazyopenconnect daemon stop all")
		os.Exit(1)
	}
	sudoArgs := append([]string{"sudo", exe}, os.Args[1:]...)
	if err := syscall.Exec(sudoPath, sudoArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to re-exec with sudo: %v\n", err)
		os.Exit(1)
	}
}

func stopAllDaemons(socketPath string) error {
	status := daemonStatus(socketPath)
	if status == daemonReachable {
		shutdownDaemon(socketPath)
		time.Sleep(150 * time.Millisecond)
	}

	procs, err := findMatchingDaemonProcs()
	if err != nil {
		return err
	}

	if len(procs) == 0 {
		if status == daemonInaccessible {
			os.Remove(socketPath)
			fmt.Println("Removed stale socket")
		} else {
			fmt.Println("No matching daemon processes found")
		}
		return nil
	}

	terminated := 0
	forceKilled := 0
	permDenied := 0

	for _, proc := range procs {
		process, err := os.FindProcess(proc.pid)
		if err != nil {
			continue
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.EPERM) {
				permDenied++
			}
			continue
		}
		terminated++

		if waitForExit(proc.pid, 1500*time.Millisecond) {
			continue
		}

		if err := process.Signal(syscall.SIGKILL); err != nil {
			continue
		}
		if waitForExit(proc.pid, 500*time.Millisecond) {
			forceKilled++
		}
	}

	if permDenied > 0 && terminated == 0 {
		return fmt.Errorf("found %d daemon process(es) but permission denied; run with sudo", permDenied)
	}

	if terminated == 0 {
		fmt.Println("No matching daemon processes found")
		return nil
	}

	fmt.Printf("Stopped %d daemon process(es)", terminated)
	if forceKilled > 0 {
		fmt.Printf(" (%d force-killed)", forceKilled)
	}
	fmt.Println()

	_ = os.Remove(socketPath)

	return nil
}

func findMatchingDaemonProcs() ([]daemonProc, error) {
	selfPID := os.Getpid()

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	exeBase := strings.ToLower(filepathBase(exe))

	allowedNames := map[string]struct{}{
		exeBase:           {},
		"lazyopenconnect": {},
		"lzcon":           {},
	}

	out, err := exec.Command("ps", "-axo", "pid=,comm=,args=").Output()
	if err != nil {
		return nil, err
	}

	seen := make(map[int]struct{})
	var matches []daemonProc

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == selfPID {
			continue
		}

		args := fields[2:]
		if len(args) < 3 || args[1] != "daemon" || args[2] != "run" {
			continue
		}

		comm := strings.ToLower(filepathBase(fields[1]))
		arg0 := strings.ToLower(filepathBase(args[0]))
		if _, ok := allowedNames[comm]; !ok {
			if _, ok := allowedNames[arg0]; !ok {
				continue
			}
		}

		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		matches = append(matches, daemonProc{pid: pid, comm: comm, args: args})
	}

	return matches, nil
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func filepathBase(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
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

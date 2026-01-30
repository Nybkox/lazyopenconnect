package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/app"
	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/presentation"
	"github.com/Nybkox/lazyopenconnect/pkg/version"
)

var (
	buildVersion = "dev"
	commit       = "none"
	date         = "unknown"
)

func main() {
	version.Current = buildVersion

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("lazyopenconnect %s (commit: %s, built: %s)\n", buildVersion, commit, date)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "update" {
		handleUpdate()
		return
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
		fmt.Println("lazyopenconnect requres sudo to run openconnect...")
		args := append([]string{"sudo", exe}, os.Args[1:]...)
		if err := syscall.Exec(sudoPath, args, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, "Requires root. Run: sudo lazyopenconnect")
			os.Exit(1)
		}
	}

	cfg, err := helpers.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	a := app.New(cfg)
	a.RenderView = presentation.Render

	p := tea.NewProgram(a, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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

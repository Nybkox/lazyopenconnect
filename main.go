package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nybkox/lazyopenconnect/pkg/app"
	"github.com/Nybkox/lazyopenconnect/pkg/controllers/helpers"
	"github.com/Nybkox/lazyopenconnect/pkg/presentation"
)

func main() {
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

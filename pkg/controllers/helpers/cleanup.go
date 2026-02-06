package helpers

import (
	"os/exec"
	"strings"

	"github.com/Nybkox/lazyopenconnect/pkg/ui"
)

type CleanupStep struct {
	Name string
	Cmd  string
	Fn   func() error
}

type CleanupResult struct {
	Step    string
	CmdStr  string
	Success bool
	Error   string
}

func RunCleanupSteps(snap *NetworkSnapshot) []CleanupResult {
	steps := PlatformCleanupSteps(snap)
	var results []CleanupResult

	for _, step := range steps {
		r := CleanupResult{Step: step.Name, CmdStr: step.Cmd}
		if err := step.Fn(); err != nil {
			r.Success = false
			r.Error = err.Error()
		} else {
			r.Success = true
		}
		results = append(results, r)
	}

	return results
}

func FormatCleanupResults(results []CleanupResult) []string {
	var lines []string
	for _, r := range results {
		lines = append(lines, r.Step+"...")
		lines = append(lines, ui.LogCommand(r.CmdStr))
		if r.Success {
			lines = append(lines, ui.LogOK("Done"))
		} else {
			lines = append(lines, ui.LogFail(r.Error))
		}
	}
	return lines
}

func runCmd(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func runCmdWithOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

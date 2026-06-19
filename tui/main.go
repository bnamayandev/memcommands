package main

import (
	"errors"
	"log"
	"memcommands/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	history, err := core.GetHistoryLines()
	if err != nil {
		log.Fatal(err)
	}

	// Shell aliases load async (see model.Init) so the UI draws immediately.
	m := New(filterSelfInvocations(history), core.AliasIndex{})

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		log.Fatal(err)
	}

	final, ok := finalModel.(model)
	if !ok || final.executed == "" {
		return
	}

	if err := shellCommand(final.executed, final.aliases).Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
}

func shellCommand(command string, aliases core.AliasIndex) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-lc", core.ExpandAliasCommand(command, aliases))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func commandName(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}

func filterSelfInvocations(commands []string) []string {
	out := make([]string, 0, len(commands))
	for _, command := range commands {
		if commandName(command) == "memcommands" {
			continue
		}
		out = append(out, command)
	}
	return out
}

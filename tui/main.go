package main

import (
	"log"
	"memcommands/core"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

	// Replace this process so the command gets sole ownership of stdin.
	if err := runShellCommand(final.executed, final.aliases); err != nil {
		log.Fatal(err)
	}
}

func runShellCommand(command string, aliases core.AliasIndex) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Bubble Tea can leave the tty non-blocking; clear it so the command's reads block.
	for _, fd := range []int{0, 1, 2} {
		_ = syscall.SetNonblock(fd, false)
	}

	args := []string{shell, "-lc", core.ExpandAliasCommand(command, aliases)}
	return syscall.Exec(shell, args, os.Environ())
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

package main

import (
	"log"
	"memcommands/core"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	sessions, err := core.ListSessions()
	if err != nil {
		log.Fatal(err)
	}

	m := New(sessions, core.AliasIndex{})

	if query := strings.TrimSpace(strings.Join(os.Args[1:], " ")); query != "" {
		m.SetInitialQuery(query)
	}

	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		log.Fatal(err)
	}

	final, ok := finalModel.(model)
	if !ok || final.attachName == "" {
		return
	}

	// Replace this process so zellij gets sole ownership of stdin.
	if err := core.AttachSession(final.attachName, final.attachCreate); err != nil {
		log.Fatal(err)
	}
}

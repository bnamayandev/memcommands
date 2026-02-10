// this is a gpt generated TUI that I put as a placeholder lowkenuinely
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	count int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.count++
		case "down", "j":
			m.count--
		}
	}

	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf(
		"Basic Bubble Tea TUI\n\nCount: %d\n\n↑/k to increment, ↓/j to decrement, q to quit\n",
		m.count,
	)
}

func main() {
	p := tea.NewProgram(model{})
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

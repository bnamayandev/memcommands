package main

import (
	"errors"
	"fmt"
	"log"
	"memcommands/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	BorderColor     lipgloss.Color
	userInput       lipgloss.Style
	indexedCommands lipgloss.Style
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.BorderColor = lipgloss.Color("36")

	s.userInput = lipgloss.NewStyle().BorderForeground(s.BorderColor).BorderStyle(lipgloss.NormalBorder()).Width(80)

	s.indexedCommands = lipgloss.NewStyle().BorderForeground(s.BorderColor).BorderStyle(lipgloss.NormalBorder()).Width(80)
	return s
}

type model struct {
	history       []string
	commands      []string
	width         int
	height        int
	selectedIndex int
	executed      string
	userInput     textinput.Model
	styles        *Styles
}

func New(commands []string) *model {
	styles := DefaultStyles()
	userInput := textinput.New()
	userInput.Placeholder = "_"
	userInput.Focus()

	m := &model{
		history:   commands,
		commands:  commands,
		userInput: userInput,
		styles:    styles,
	}

	m.refreshCommands()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			fmt.Print("\033[H\033[2J") // clear screen and move cursor to top-left
			return m, tea.Quit

		case "enter":
			cmd := m.selectedCommand()
			if cmd == "" {
				return m, nil
			}

			m.executed = cmd

			fmt.Print("\033[H\033[2J") // clear screen and move cursor to top-left
			return m, tea.Quit

		case "ctrl+j":
			if len(m.commands) > 0 && m.selectedIndex < min(9, len(m.commands)-1) {
				m.selectedIndex++
			}
			return m, nil

		case "ctrl+k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil
		}
	}

	m.userInput, cmd = m.userInput.Update(msg)
	m.refreshCommands()

	return m, cmd
}

func (m model) selectedCommand() string {
	if len(m.commands) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.commands) {
		return ""
	}
	return m.commands[m.selectedIndex]
}

func shellCommand(command string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-lc", command)
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

func (m *model) refreshCommands() {
	scored := core.GetFuzzyScoreList(m.history, m.userInput.Value())

	m.commands = m.commands[:0]
	for _, s := range scored {
		m.commands = append(m.commands, s.Command)
	}

	if len(m.commands) == 0 {
		m.selectedIndex = 0
		return
	}

	if m.selectedIndex >= len(m.commands) {
		m.selectedIndex = len(m.commands) - 1
	}
}

func (m model) renderIndexedCommands() string {
	lines := []string{}

	limit := min(10, len(m.commands))
	for i := 0; i < limit; i++ {
		line := fmt.Sprintf("%d. %s", i+1, m.commands[i])

		if i == m.selectedIndex {
			line = "> " + line
		} else {
			line = "  " + line
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) View() string {
	selectedCommand := m.selectedCommand()
	if selectedCommand == "" {
		selectedCommand = "No commands loaded"
	}

	indexedBlock := m.styles.indexedCommands.Render(m.renderIndexedCommands())

	return lipgloss.JoinVertical(
		lipgloss.Center,
		selectedCommand,
		m.styles.userInput.Render(m.userInput.View()),
		indexedBlock,
	)
}

func main() {
	history, err := core.GetHistoryLines()
	if err != nil {
		log.Fatal(err)
		return
	}

	m := New(filterSelfInvocations(history))

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	if m, ok := finalModel.(model); ok && m.executed != "" {
		if err := shellCommand(m.executed).Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			log.Fatal(err)
		}
	}
}

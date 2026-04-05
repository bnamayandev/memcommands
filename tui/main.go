package main

import (
	"log"
	"memcommands/core"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	BorderColor lipgloss.Color
	userInput lipgloss.Style
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.BorderColor = lipgloss.Color("36")
	s.userInput = lipgloss.NewStyle().BorderForeground(s.BorderColor).BorderStyle(lipgloss.NormalBorder()).Width(80)

	return s
}

type model struct {
	commands []string
	width int
	height int
	selectedIndex int16
	userInput textinput.Model
	styles *Styles
}

func New(commands []string) *model {
	styles := DefaultStyles()
	userInput := textinput.New()
	userInput.Placeholder = "_"
	userInput.Focus()
	return &model{commands: commands, userInput: userInput, styles: styles}
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
		case "ctrl+c" :
			return m, tea.Quit
		}
	}

	m.userInput, cmd = m.userInput.Update(msg)
	return m, cmd
}

func (m model) View() string {

	return lipgloss.JoinVertical(
			lipgloss.Center,
			m.commands[m.selectedIndex],
			m.styles.userInput.Render(m.userInput.View()),
		)
}

func main() {
	history, err := core.GetHistoryLines()

	if err != nil{
		log.Fatal(err)
		return
	}

	m := New(history);

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}


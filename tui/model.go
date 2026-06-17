package main

import (
	"memcommands/core"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const maxResults = 10

type focusState int

const (
	focusSearch focusState = iota
	focusResults
)

type vimMode int

const (
	modeNormal vimMode = iota
	modeInsert
)

type model struct {
	history       []string
	commands      []string
	aliases       core.AliasIndex
	width         int
	height        int
	selectedIndex int
	scrollOffset  int
	executed      string
	userInput     textinput.Model
	styles        *Styles

	focus      focusState
	mode       vimMode
	editBuffer []rune
	cursor     int
	pending    string
}

func New(commands []string, aliases core.AliasIndex) *model {
	input := textinput.New()
	input.Placeholder = "_"
	input.Focus()

	m := &model{
		history:   commands,
		commands:  commands,
		aliases:   aliases,
		userInput: input,
		styles:    DefaultStyles(),
		focus:     focusSearch,
	}
	m.refreshCommands()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.focus == focusSearch {
			return m.updateSearch(msg)
		}
		return m.updateResults(msg)
	}

	if m.focus == focusSearch {
		var cmd tea.Cmd
		m.userInput, cmd = m.userInput.Update(msg)
		m.refreshCommands()
		return m, cmd
	}
	return m, nil
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "enter":
		return m.run(m.firstCommand())
	case "ctrl+j", "ctrl+n", "ctrl+k", "ctrl+p":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.enterResults()
		return m, nil
	}

	var cmd tea.Cmd
	m.userInput, cmd = m.userInput.Update(msg)
	m.refreshCommands()
	return m, cmd
}

func (m *model) enterResults() {
	m.focus = focusResults
	m.mode = modeNormal
	m.pending = ""
	m.userInput.Blur()

	if m.selectedIndex >= len(m.commands) {
		m.selectedIndex = len(m.commands) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	m.ensureVisible()
	m.loadEditBuffer()
}

// ensureVisible scrolls the visible window so the selected row stays in view.
func (m *model) ensureVisible() {
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+maxResults {
		m.scrollOffset = m.selectedIndex - maxResults + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *model) leaveResults() {
	m.focus = focusSearch
	m.mode = modeNormal
	m.pending = ""
	m.userInput.Focus()
}

func (m *model) loadEditBuffer() {
	m.editBuffer = nil
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
		m.editBuffer = []rune(m.commands[m.selectedIndex])
	}
	m.cursor = 0
}

func (m *model) refreshCommands() {
	scored := core.GetFuzzyScoreList(m.history, m.userInput.Value(), m.aliases)

	m.commands = m.commands[:0]
	for _, s := range scored {
		m.commands = append(m.commands, s.Command)
	}

	if len(m.commands) == 0 {
		m.selectedIndex = 0
		m.scrollOffset = 0
		return
	}
	if m.selectedIndex >= len(m.commands) {
		m.selectedIndex = len(m.commands) - 1
	}
	m.ensureVisible()
}

func (m model) firstCommand() string {
	if len(m.commands) == 0 {
		return ""
	}
	return m.commands[0]
}

func (m model) run(command string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(command) == "" {
		return m, nil
	}
	m.executed = command
	clearScreen()
	return m, tea.Quit
}

func (m model) quit() (tea.Model, tea.Cmd) {
	clearScreen()
	return m, tea.Quit
}

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

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

type Styles struct {
	FocusedBorder lipgloss.Color
	BlurredBorder lipgloss.Color
	userInput     lipgloss.Style
	results       lipgloss.Style
	selectedRow   lipgloss.Style
	cursor        lipgloss.Style
	status        lipgloss.Style
}

func DefaultStyles() *Styles {
	s := new(Styles)
	s.FocusedBorder = lipgloss.Color("#89B4FA")
	s.BlurredBorder = lipgloss.Color("#45475A")

	s.userInput = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	s.results = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	s.selectedRow = lipgloss.NewStyle().Background(lipgloss.Color("#313244")).Bold(true)
	s.cursor = lipgloss.NewStyle().Reverse(true)
	s.status = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	return s
}

type model struct {
	history       []string
	commands      []string
	aliases       core.AliasIndex
	width         int
	height        int
	selectedIndex int
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
	styles := DefaultStyles()
	userInput := textinput.New()
	userInput.Placeholder = "_"
	userInput.Focus()

	m := &model{
		history:   commands,
		commands:  commands,
		aliases:   aliases,
		userInput: userInput,
		styles:    styles,
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
	m.loadEditBuffer()
}

func (m *model) leaveResults() {
	m.focus = focusSearch
	m.mode = modeNormal
	m.pending = ""
	m.userInput.Focus()
}

func (m *model) loadEditBuffer() {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
		m.editBuffer = []rune(m.commands[m.selectedIndex])
	} else {
		m.editBuffer = nil
	}
	m.cursor = 0
}

func (m model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.quit()
	}

	if m.mode == modeInsert {
		return m.updateInsert(msg)
	}
	return m.updateNormal(msg)
}

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.cursor--
		m.clampCursor()
		return m, nil

	case tea.KeyEnter:
		return m.run(string(m.editBuffer))

	case tea.KeyBackspace:
		if m.cursor > 0 {
			m.editBuffer = append(m.editBuffer[:m.cursor-1], m.editBuffer[m.cursor:]...)
			m.cursor--
		}
		return m, nil

	case tea.KeyLeft:
		m.cursor--
		m.clampCursor()
		return m, nil

	case tea.KeyRight:
		m.cursor++
		m.clampCursor()
		return m, nil

	case tea.KeySpace:
		m.insertRunes([]rune{' '})
		return m, nil

	case tea.KeyRunes:
		m.insertRunes(msg.Runes)
		return m, nil
	}

	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pending != "" {
		return m.applyOperator(key), nil
	}

	switch key {
	case "esc":
		m.leaveResults()
		return m, nil

	case "enter":
		return m.run(string(m.editBuffer))

	case "j", "ctrl+j", "ctrl+n":
		if m.selectedIndex < min(9, len(m.commands)-1) {
			m.selectedIndex++
			m.loadEditBuffer()
		}
		return m, nil

	case "k", "ctrl+k", "ctrl+p":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.loadEditBuffer()
		}
		return m, nil

	case "h":
		m.cursor--
		m.clampCursor()
		return m, nil

	case "l":
		m.cursor++
		m.clampCursor()
		return m, nil

	case "0":
		m.cursor = 0
		return m, nil

	case "$":
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
		return m, nil

	case "w":
		m.cursor = nextWordStart(m.editBuffer, m.cursor)
		m.clampCursor()
		return m, nil

	case "b":
		m.cursor = prevWordStart(m.editBuffer, m.cursor)
		m.clampCursor()
		return m, nil

	case "i":
		m.mode = modeInsert
		return m, nil

	case "a":
		m.mode = modeInsert
		m.cursor++
		if m.cursor > len(m.editBuffer) {
			m.cursor = len(m.editBuffer)
		}
		return m, nil

	case "A":
		m.mode = modeInsert
		m.cursor = len(m.editBuffer)
		return m, nil

	case "I":
		m.mode = modeInsert
		m.cursor = 0
		return m, nil

	case "x":
		if len(m.editBuffer) > 0 && m.cursor < len(m.editBuffer) {
			m.editBuffer = append(m.editBuffer[:m.cursor], m.editBuffer[m.cursor+1:]...)
			m.clampCursor()
		}
		return m, nil

	case "d", "c", "y":
		m.pending = key
		return m, nil

	case "p":
		m.paste(true)
		return m, nil

	case "P":
		m.paste(false)
		return m, nil
	}

	return m, nil
}

// applyOperator completes a pending operator (d/c/y) with a motion key.
func (m model) applyOperator(key string) model {
	op := m.pending
	m.pending = ""

	start, end := m.cursor, m.cursor

	switch key {
	case op: // dd, cc, yy -> whole line
		start, end = 0, len(m.editBuffer)
	case "w":
		end = nextWordStart(m.editBuffer, m.cursor)
	case "$":
		end = len(m.editBuffer)
	case "0":
		start = 0
	case "b":
		start = prevWordStart(m.editBuffer, m.cursor)
	default:
		return m // unknown motion: cancel
	}

	if start > end {
		start, end = end, start
	}

	switch op {
	case "y":
		writeClipboard(string(m.editBuffer[start:end]))
	case "d":
		m.editBuffer = append(m.editBuffer[:start], m.editBuffer[end:]...)
		m.cursor = start
		m.clampCursor()
	case "c":
		m.editBuffer = append(m.editBuffer[:start], m.editBuffer[end:]...)
		m.cursor = start
		m.mode = modeInsert
	}

	return m
}

func (m *model) insertRunes(rs []rune) {
	buf := make([]rune, 0, len(m.editBuffer)+len(rs))
	buf = append(buf, m.editBuffer[:m.cursor]...)
	buf = append(buf, rs...)
	buf = append(buf, m.editBuffer[m.cursor:]...)
	m.editBuffer = buf
	m.cursor += len(rs)
}

func (m *model) paste(after bool) {
	text, err := clipboard.ReadAll()
	if err != nil || text == "" {
		return
	}
	text = strings.ReplaceAll(text, "\n", " ")
	rs := []rune(text)

	at := m.cursor
	if after && len(m.editBuffer) > 0 {
		at++
	}
	if at > len(m.editBuffer) {
		at = len(m.editBuffer)
	}

	buf := make([]rune, 0, len(m.editBuffer)+len(rs))
	buf = append(buf, m.editBuffer[:at]...)
	buf = append(buf, rs...)
	buf = append(buf, m.editBuffer[at:]...)
	m.editBuffer = buf
	m.cursor = at + len(rs) - 1
	m.clampCursor()
}

func (m *model) clampCursor() {
	maxCursor := len(m.editBuffer)
	if m.mode == modeNormal {
		maxCursor = len(m.editBuffer) - 1
	}
	if maxCursor < 0 {
		maxCursor = 0
	}
	if m.cursor > maxCursor {
		m.cursor = maxCursor
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
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
	fmt.Print("\033[H\033[2J") // clear screen and move cursor to top-left
	return m, tea.Quit
}

func (m model) quit() (tea.Model, tea.Cmd) {
	fmt.Print("\033[H\033[2J") // clear screen and move cursor to top-left
	return m, tea.Quit
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func nextWordStart(rs []rune, i int) int {
	n := len(rs)
	if i >= n {
		return n
	}
	for i < n && !isSpace(rs[i]) {
		i++
	}
	for i < n && isSpace(rs[i]) {
		i++
	}
	return i
}

func prevWordStart(rs []rune, i int) int {
	if i <= 0 {
		return 0
	}
	i--
	for i > 0 && isSpace(rs[i]) {
		i--
	}
	for i > 0 && !isSpace(rs[i-1]) {
		i--
	}
	return i
}

func writeClipboard(text string) {
	_ = clipboard.WriteAll(text)
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

func (m *model) refreshCommands() {
	scored := core.GetFuzzyScoreList(m.history, m.userInput.Value(), m.aliases)

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

func (m model) renderEditLine() string {
	var b strings.Builder
	for i := 0; i < len(m.editBuffer); i++ {
		if i == m.cursor {
			b.WriteString(m.styles.cursor.Render(string(m.editBuffer[i])))
		} else {
			b.WriteString(string(m.editBuffer[i]))
		}
	}
	if m.cursor >= len(m.editBuffer) {
		b.WriteString(m.styles.cursor.Render(" "))
	}
	return b.String()
}

func (m model) renderIndexedCommands() string {
	lines := []string{}

	limit := min(10, len(m.commands))
	for i := 0; i < limit; i++ {
		selected := i == m.selectedIndex

		var text string
		if selected && m.focus == focusResults {
			text = m.renderEditLine()
		} else {
			text = m.commands[i]
		}

		line := fmt.Sprintf("%d. %s", i+1, text)
		if selected {
			line = "> " + line
		} else {
			line = "  " + line
		}

		if selected && m.focus == focusResults {
			line = m.styles.selectedRow.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m model) statusLine() string {
	switch {
	case m.focus == focusSearch:
		return "SEARCH  enter: run top result   ctrl+j/n: focus results"
	case m.mode == modeInsert:
		return "-- INSERT --  esc: normal   enter: run"
	default:
		return "NORMAL  j/k: move  h/l 0 $ w b: cursor  i/a: edit  x dd dw cw: delete  yy/y$: yank  p: paste  enter: run  esc: search"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) View() string {
	contentWidth := max(0, m.width-2)

	searchBorder := m.styles.BlurredBorder
	resultsBorder := m.styles.BlurredBorder
	if m.focus == focusSearch {
		searchBorder = m.styles.FocusedBorder
	} else {
		resultsBorder = m.styles.FocusedBorder
	}

	searchBlock := m.styles.userInput.
		BorderForeground(searchBorder).
		Width(contentWidth).
		Render(m.userInput.View())

	resultsBlock := m.styles.results.
		BorderForeground(resultsBorder).
		Width(contentWidth).
		Render(m.renderIndexedCommands())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		searchBlock,
		resultsBlock,
		m.styles.status.Render(m.statusLine()),
	)
}

func main() {
	history, err := core.GetHistoryLines()
	if err != nil {
		log.Fatal(err)
		return
	}

	m := New(filterSelfInvocations(history), core.LoadAliases())

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
		if err := shellCommand(m.executed, m.aliases).Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			log.Fatal(err)
		}
	}
}

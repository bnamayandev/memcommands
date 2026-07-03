package main

import (
	"memcommands/core"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	modeVisual
)

type model struct {
	history       []string
	corpus        *core.Corpus
	commands      []string
	aliases       core.AliasIndex
	width         int
	height        int
	selectedIndex int
	scrollOffset  int
	executed      string
	userInput     textinput.Model
	styles        *Styles

	focus        focusState
	mode         vimMode
	editBuffer   []rune
	cursor       int
	pending      string
	pendingCount int
	count        string
	gPending     bool
	// pendingFind arms an operator awaiting an f/F/t/T target, e.g. `dt,`.
	pendingFind string
	// pendingObj / objPending arm a text-object char for an operator / visual select.
	pendingObj string
	objPending string
	// rPending arms `r`: the next key replaces the char(s) under the cursor.
	rPending bool
	// findPending holds an armed f/F/t/T awaiting its target; findCount is its count; lastFindCmd/lastFindChar back ;/, repeats.
	findPending  string
	findCount    int
	lastFindCmd  string
	lastFindChar rune
	// visualAnchor is the fixed end of the visual selection; the other end follows
	// the cursor.
	visualAnchor int

	// The alias label is edited inline as a protected [bracket] prefix on the
	// command buffer; aliasLen is how many leading editBuffer runes belong to it.
	aliasLen int
	// editAlias marks that a boundary insert (cursor at aliasLen) should grow the
	// alias rather than the command; set when entering insert from the alias side.
	editAlias   bool
	userAliases map[string]string

	deleted   map[string]string
	undoStack []string
	// edited maps a command's normalized key to its rewritten text, so edits
	// survive navigation and (after :w) restarts.
	edited map[string]string

	// vim-style ":" command line; commandLine is the text after the colon.
	commandMode bool
	commandLine string
	statusMsg   string // transient message shown until the next normal-mode key
	dirty       bool   // staged aliases/deletions not yet written to disk

	// showHelp overlays the keybindings cheat-sheet, toggled with "?".
	showHelp bool

	// confirmQuit shows the unsaved-changes prompt raised on ctrl+c.
	confirmQuit bool

	// aliasFilter (ctrl+a) shows only aliased commands.
	aliasFilter bool
}

func New(commands []string, aliases core.AliasIndex) *model {
	input := textinput.New()
	input.Placeholder = "search history…"
	input.Prompt = "❯ "
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colBlue))
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colBlue))
	input.Focus()

	userAliases := core.LoadUserAliases()
	aliases.ByFullCommand = core.BuildUserAliasIndex(userAliases)

	deleted := make(map[string]string)
	for _, cmd := range core.LoadDeletedCommands() {
		deleted[core.NormalizeCommandKey(cmd)] = cmd
	}

	m := &model{
		history:     commands,
		commands:    nil,
		aliases:     aliases,
		userInput:   input,
		userAliases: userAliases,
		deleted:     deleted,
		edited:      core.LoadEditedCommands(),
		styles:      DefaultStyles(),
		focus:       focusSearch,
	}
	m.corpus = core.NewCorpus(m.history, m.aliases)
	m.refreshCommands()
	return m
}

func (m *model) SetInitialQuery(query string) {
	m.userInput.SetValue(query)
	m.userInput.CursorEnd()
	m.refreshCommands()
}

// aliasesLoadedMsg carries shell aliases loaded async, so the interactive-shell
// spawn never blocks the first draw.
type aliasesLoadedMsg struct {
	byAlias   map[string]string
	byCommand map[string][]string
}

func loadShellAliases() tea.Msg {
	byAlias, byCommand := core.LoadShellAliases()
	return aliasesLoadedMsg{byAlias: byAlias, byCommand: byCommand}
}

func (m model) Init() tea.Cmd {
	return loadShellAliases
}

// contentWidth is the width inside the borders (1 column per side).
func (m model) contentWidth() int {
	return max(0, m.width-2)
}

// innerWidth is contentWidth minus horizontal padding.
func (m model) innerWidth() int {
	return max(0, m.contentWidth()-2*hPad)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.userInput.Width = max(0, m.innerWidth()-3)
		return m, nil
	case aliasesLoadedMsg:
		m.aliases.ByAlias = msg.byAlias
		m.aliases.ByCommand = msg.byCommand
		m.corpus = core.NewCorpus(m.history, m.aliases)
		m.refreshCommands()
		return m, nil
	case tea.KeyMsg:
		if m.confirmQuit {
			return m.updateConfirmQuit(msg)
		}
		// While the help overlay is up, any key dismisses it.
		if m.showHelp {
			if msg.String() == "ctrl+c" {
				return m.requestQuit()
			}
			m.showHelp = false
			return m, nil
		}
		if m.commandMode {
			return m.updateCommand(msg)
		}
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
	case "ctrl+c", "esc":
		return m.requestQuit()
	case "enter":
		return m.run(m.firstCommand())
	case "ctrl+j", "ctrl+n", "ctrl+k", "ctrl+p", "down", "up":
		if len(m.commands) == 0 {
			return m, nil
		}
		m.enterResults()
		return m, nil
	case "ctrl+a":
		m.toggleAliasFilter()
		return m, nil
	case "?":
		// Open help on "?" only with an empty query, so it stays typeable mid-search.
		if m.userInput.Value() == "" {
			m.showHelp = true
			return m, nil
		}
	}

	before := m.userInput.Value()
	var cmd tea.Cmd
	m.userInput, cmd = m.userInput.Update(msg)
	// Editing the query resets the selection to the top result.
	if m.userInput.Value() != before {
		m.selectedIndex = 0
		m.scrollOffset = 0
	}
	m.refreshCommands()
	return m, cmd
}

func (m *model) enterResults() {
	m.focus = focusResults
	m.mode = modeNormal
	m.pending = ""
	m.count = ""
	m.gPending = false
	m.rPending = false
	m.findPending = ""
	m.pendingFind = ""
	m.pendingObj = ""
	m.objPending = ""
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
	m.commitEdit()
	m.focus = focusSearch
	m.mode = modeNormal
	m.pending = ""
	m.count = ""
	m.gPending = false
	m.rPending = false
	m.findPending = ""
	m.pendingFind = ""
	m.pendingObj = ""
	m.objPending = ""
	m.userInput.Focus()
}

// loadEditBuffer loads the selected command, prefixed by its alias as a tracked [bracket] region.
func (m *model) loadEditBuffer() {
	m.editBuffer = nil
	m.aliasLen = 0
	m.editAlias = false
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
		cmd := m.commands[m.selectedIndex]
		var buf []rune
		if labels := core.AliasesForCommand(cmd, m.aliases); len(labels) > 0 {
			buf = []rune(labels[0])
			m.aliasLen = len(buf)
		}
		m.editBuffer = append(buf, []rune(m.resolve(cmd))...)
	}
	m.cursor = 0
}

// commandText returns the edit buffer without its leading alias region.
func (m model) commandText() string {
	if m.aliasLen > len(m.editBuffer) {
		return ""
	}
	return string(m.editBuffer[m.aliasLen:])
}

// resolve returns the staged edit for a command, else the command itself.
func (m model) resolve(command string) string {
	if text, ok := m.edited[core.NormalizeCommandKey(command)]; ok {
		return text
	}
	return command
}

// commitEdit stages the edit buffer against the selected command before leaving
// it (navigate, run, :command). A buffer matching the original clears the
// overlay; an empty buffer is ignored so a command can't be blanked out.
func (m *model) commitEdit() {
	if m.focus != focusResults || m.selectedIndex < 0 || m.selectedIndex >= len(m.commands) {
		return
	}
	original := m.commands[m.selectedIndex]
	m.commitAlias(original)

	key := core.NormalizeCommandKey(original)
	text := m.commandText()

	if strings.TrimSpace(text) == "" {
		return
	}
	if text == original {
		if _, ok := m.edited[key]; ok {
			delete(m.edited, key)
			m.dirty = true
		}
		return
	}
	if m.edited == nil {
		m.edited = make(map[string]string)
	}
	if m.edited[key] != text {
		m.edited[key] = text
		m.dirty = true
	}
}

func (m *model) refreshCommands() {
	scored := m.corpus.Search(m.userInput.Value())

	m.commands = m.commands[:0]
	for _, s := range scored {
		if _, ok := m.deleted[core.NormalizeCommandKey(s.Command)]; ok {
			continue
		}
		if m.aliasFilter && !m.isAliased(s.Command) {
			continue
		}
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

// isAliased reports whether a command carries a user-defined alias label.
func (m model) isAliased(command string) bool {
	return len(core.AliasesForCommand(command, m.aliases)) > 0
}

// toggleAliasFilter flips the aliased-only filter and rebuilds the result list.
func (m *model) toggleAliasFilter() {
	if m.focus == focusResults {
		m.commitEdit()
	}
	m.aliasFilter = !m.aliasFilter
	m.refreshCommands()
	if m.focus == focusResults {
		m.loadEditBuffer()
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
	// Running exits the app, so persist staged curation on the way out.
	m.save()
	m.executed = command
	clearScreen()
	return m, tea.Quit
}

func (m model) quit() (tea.Model, tea.Cmd) {
	clearScreen()
	return m, tea.Quit
}

// requestQuit prompts before quitting when there are unsaved changes.
func (m model) requestQuit() (tea.Model, tea.Cmd) {
	m.commitEdit()
	if m.dirty {
		m.confirmQuit = true
		m.showHelp = false
		return m, nil
	}
	return m.quit()
}

func (m model) updateConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s", "enter":
		m.save()
		return m.quit()
	case "d":
		return m.quit()
	case "esc", "ctrl+c":
		m.confirmQuit = false
	}
	return m, nil
}

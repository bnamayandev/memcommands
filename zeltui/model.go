package main

import (
	"memcommands/core"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxResults = 10

// appName scopes this tool's local config (aliases, pins) to its own
// ~/.config/zelcommands/ directory, separate from memcommands.
const appName = "zelcommands"

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

// confirmKind identifies which destructive action a confirm overlay is armed
// for. Unlike memcommands' local, undoable dd, kill/delete here are real
// zellij calls, so they get a confirmation step and no undo.
type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmKill
	confirmDelete
)

type model struct {
	sessions      []core.Session
	byName        map[string]core.Session
	corpus        *core.Corpus
	aliases       core.AliasIndex
	commands      []string // session names, in fuzzy-match/pin order
	width         int
	height        int
	selectedIndex int
	scrollOffset  int
	attachName    string // set on quit-to-attach; empty means the app just quit
	attachCreate  bool
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
	// pendingG arms an operator awaiting a bare-g target (`ge`/`gE`), e.g. `dge`.
	pendingG bool
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

	// yankActive highlights the [yankStart, yankEnd) span just copied to the
	// clipboard until its flash timer fires; yankGen invalidates a stale timer
	// when a newer yank arms before the old one clears.
	yankActive         bool
	yankStart, yankEnd int
	yankGen            int

	// The alias label is edited inline as a protected [bracket] prefix on the
	// name buffer; aliasLen is how many leading editBuffer runes belong to it.
	aliasLen int
	// editAlias marks that a boundary insert (cursor at aliasLen) should grow the
	// alias rather than the session name.
	editAlias   bool
	userAliases map[string]string

	// pinned favorites float to the top of the results list; keyed by normalized name.
	pinned map[string]string

	// vim-style ":" command line; commandLine is the text after the colon.
	commandMode bool
	commandLine string
	statusMsg   string // transient message shown until the next normal-mode key
	dirty       bool   // staged aliases/pins not yet written to disk

	// showHelp overlays the keybindings cheat-sheet, toggled with "?".
	showHelp bool

	// confirmQuit shows the unsaved-changes prompt raised on ctrl+c.
	confirmQuit bool

	// confirm arms the kill/delete confirmation overlay; confirmTargets holds
	// the session name(s) it would act on.
	confirm        confirmKind
	confirmTargets []string

	// aliasFilter (ctrl+a) shows only aliased sessions.
	aliasFilter bool
}

func New(sessions []core.Session, aliases core.AliasIndex) *model {
	input := textinput.New()
	input.Placeholder = "search sessions…"
	input.Prompt = "❯ "
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colBlue))
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0))
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colBlue))
	input.Focus()

	userAliases := core.LoadUserAliases(appName)
	aliases.ByFullCommand = core.BuildUserAliasIndex(userAliases)

	pinned := make(map[string]string)
	for _, name := range core.LoadPinnedCommands(appName) {
		pinned[core.NormalizeCommandKey(name)] = name
	}

	m := &model{
		sessions:    sessions,
		aliases:     aliases,
		userInput:   input,
		userAliases: userAliases,
		pinned:      pinned,
		styles:      DefaultStyles(),
		focus:       focusSearch,
	}
	m.indexSessions()
	m.corpus = core.NewCorpus(m.sessionNames(), m.aliases)
	m.refreshCommands()
	return m
}

// indexSessions rebuilds byName from the current session list.
func (m *model) indexSessions() {
	m.byName = make(map[string]core.Session, len(m.sessions))
	for _, s := range m.sessions {
		m.byName[s.Name] = s
	}
}

func (m *model) SetInitialQuery(query string) {
	m.userInput.SetValue(query)
	m.userInput.CursorEnd()
	m.refreshCommands()
}

func (m model) sessionNames() []string {
	names := make([]string, len(m.sessions))
	for i, s := range m.sessions {
		names[i] = s.Name
	}
	return names
}

func (m model) Init() tea.Cmd {
	return nil
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
	case yankFadeMsg:
		if msg.gen == m.yankGen {
			m.yankActive = false
		}
		return m, nil
	case tea.KeyMsg:
		if m.confirmQuit {
			return m.updateConfirmQuit(msg)
		}
		if m.confirm != confirmNone {
			return m.updateConfirm(msg)
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
		return m.attachSelected(m.firstCommand(), false)
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
	m.pendingG = false
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
	m.commitRename()
	m.focus = focusSearch
	m.mode = modeNormal
	m.pending = ""
	m.count = ""
	m.gPending = false
	m.pendingG = false
	m.rPending = false
	m.findPending = ""
	m.pendingFind = ""
	m.pendingObj = ""
	m.objPending = ""
	m.userInput.Focus()
}

// loadEditBuffer loads the selected session's name, prefixed by its alias as a tracked [bracket] region.
func (m *model) loadEditBuffer() {
	m.editBuffer = nil
	m.aliasLen = 0
	m.editAlias = false
	m.yankActive = false
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
		name := m.commands[m.selectedIndex]
		var buf []rune
		if labels := core.AliasesForCommand(name, m.aliases); len(labels) > 0 {
			buf = []rune(labels[0])
			m.aliasLen = len(buf)
		}
		m.editBuffer = append(buf, []rune(name)...)
	}
	m.cursor = 0
}

// nameText returns the edit buffer without its leading alias region.
func (m model) nameText() string {
	if m.aliasLen > len(m.editBuffer) {
		return ""
	}
	return string(m.editBuffer[m.aliasLen:])
}

// selectedSession returns the session under the cursor, if any.
func (m model) selectedSession() (core.Session, bool) {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.commands) {
		return core.Session{}, false
	}
	s, ok := m.byName[m.commands[m.selectedIndex]]
	return s, ok
}

// commitRename stages the alias region like memcommands, then — unlike
// memcommands — commits a changed name immediately as a real zellij rename
// call, since the buffer here is live state, not a local override. An EXITED
// session (no live IPC socket) or a failed call reverts the buffer.
func (m *model) commitRename() {
	if m.focus != focusResults {
		return
	}
	session, ok := m.selectedSession()
	if !ok {
		return
	}
	m.commitAlias(session.Name)

	newName := strings.TrimSpace(m.nameText())
	if newName == "" || newName == session.Name {
		return
	}

	if session.Exited {
		m.statusMsg = "can't rename an exited session — resurrect it first (enter)"
		m.loadEditBuffer()
		return
	}

	if err := core.RenameSession(session.Name, newName); err != nil {
		m.statusMsg = err.Error()
		m.loadEditBuffer()
		return
	}

	m.migrateLocalMetadata(session.Name, newName)
	m.refreshSessions()
	m.selectedIndex = m.indexOfCommand(newName)
	m.ensureVisible()
	m.loadEditBuffer()
}

// refreshSessions re-fetches the live session list from zellij and rebuilds
// the corpus, preserving the current query and filter.
func (m *model) refreshSessions() {
	sessions, err := core.ListSessions()
	if err != nil {
		m.statusMsg = err.Error()
		return
	}
	m.sessions = sessions
	m.indexSessions()
	m.corpus = core.NewCorpus(m.sessionNames(), m.aliases)
	m.refreshCommands()
}

func (m *model) refreshCommands() {
	scored := m.corpus.Search(m.userInput.Value())

	m.commands = m.commands[:0]
	var rest []string
	for _, s := range scored {
		if m.aliasFilter && !m.isAliased(s.Command) {
			continue
		}
		key := core.NormalizeCommandKey(s.Command)
		if _, ok := m.pinned[key]; ok {
			m.commands = append(m.commands, s.Command)
			continue
		}
		rest = append(rest, s.Command)
	}
	m.commands = append(m.commands, rest...)

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

// isAliased reports whether a session carries a user-defined alias label.
func (m model) isAliased(name string) bool {
	return len(core.AliasesForCommand(name, m.aliases)) > 0
}

// isPinned reports whether a session is a pinned favorite.
func (m model) isPinned(name string) bool {
	_, ok := m.pinned[core.NormalizeCommandKey(name)]
	return ok
}

// togglePin pins or unpins the selected session, then rebuilds the list while
// keeping the same session under the cursor as it floats to (or from) the top.
func (m *model) togglePin() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.commands) {
		return
	}
	m.commitRename()
	name := m.commands[m.selectedIndex]
	key := core.NormalizeCommandKey(name)
	if m.pinned == nil {
		m.pinned = make(map[string]string)
	}
	if _, ok := m.pinned[key]; ok {
		delete(m.pinned, key)
	} else {
		m.pinned[key] = name
	}
	m.dirty = true

	m.refreshCommands()
	m.selectedIndex = m.indexOfCommand(name)
	m.ensureVisible()
	m.loadEditBuffer()
}

// indexOfCommand returns the row of a session by its normalized key, or 0.
func (m model) indexOfCommand(name string) int {
	key := core.NormalizeCommandKey(name)
	for i, c := range m.commands {
		if core.NormalizeCommandKey(c) == key {
			return i
		}
	}
	return 0
}

// toggleAliasFilter flips the aliased-only filter and rebuilds the result list.
func (m *model) toggleAliasFilter() {
	if m.focus == focusResults {
		m.commitRename()
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

// attachSelected quits the app and arms it to exec into `zellij attach`
// (creating the session first when create is set).
func (m model) attachSelected(name string, create bool) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(name) == "" {
		return m, nil
	}
	m.save()
	m.attachName = name
	m.attachCreate = create
	clearScreen()
	return m, tea.Quit
}

func (m model) quit() (tea.Model, tea.Cmd) {
	clearScreen()
	return m, tea.Quit
}

// requestQuit prompts before quitting when there are unsaved changes.
func (m model) requestQuit() (tea.Model, tea.Cmd) {
	m.commitRename()
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

package main

import (
	"fmt"
	"memcommands/core"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func (m model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.quit()
	}
	switch m.mode {
	case modeInsert:
		return m.updateInsert(msg)
	case modeVisual:
		return m.updateVisual(msg)
	}
	return m.updateNormal(msg)
}

// visualRange returns the half-open [start, end) rune span currently selected
// in visual mode. The selection is inclusive of the character under the cursor.
func (m model) visualRange() (int, int) {
	start, end := m.visualAnchor, m.cursor
	if start > end {
		start, end = end, start
	}
	end++
	if end > len(m.editBuffer) {
		end = len(m.editBuffer)
	}
	if start < 0 {
		start = 0
	}
	return start, end
}

func (m model) updateVisual(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && m.count != "") {
		m.count += key
		return m, nil
	}

	count := m.takeCount()

	switch key {
	case "esc":
		m.mode = modeNormal
		m.clampCursor()
	case "enter":
		return m.run(string(m.editBuffer))
	case "h":
		m.cursor -= count
		m.clampCursor()
	case "l":
		m.cursor += count
		m.clampCursor()
	case "0":
		m.cursor = 0
	case "$":
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
	case "w":
		for n := 0; n < count; n++ {
			m.cursor = nextWordStart(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "b":
		for n := 0; n < count; n++ {
			m.cursor = prevWordStart(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "y":
		start, end := m.visualRange()
		writeClipboard(string(m.editBuffer[start:end]))
		m.cursor = start
		m.mode = modeNormal
		m.clampCursor()
	case "d", "x":
		start, end := m.visualRange()
		m.editBuffer = append(m.editBuffer[:start], m.editBuffer[end:]...)
		m.cursor = start
		m.mode = modeNormal
		m.clampCursor()
	case "c":
		start, end := m.visualRange()
		m.editBuffer = append(m.editBuffer[:start], m.editBuffer[end:]...)
		m.cursor = start
		m.mode = modeInsert
	}
	return m, nil
}

// updateAlias drives the floating alias box: every key feeds the input except
// enter (save) and esc (cancel).
func (m model) updateAlias(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "esc":
		m.closeAlias()
		return m, nil
	case "enter":
		if m.saveAlias() {
			m.closeAlias()
		}
		return m, nil
	}

	m.aliasError = ""
	var cmd tea.Cmd
	m.aliasInput, cmd = m.aliasInput.Update(msg)
	return m, cmd
}

// saveAlias persists the typed alias for the targeted command and rebuilds the
// search index so it can be found immediately.
func (m *model) saveAlias() bool {
	alias := strings.TrimSpace(m.aliasInput.Value())
	target := strings.Join(strings.Fields(strings.TrimSpace(m.aliasTarget)), " ")
	if alias == "" || target == "" {
		return false
	}

	if existing, ok := m.userAliases[alias]; ok && existing != target {
		m.aliasError = fmt.Sprintf("alias %q already in use", alias)
		return false
	}

	if m.userAliases == nil {
		m.userAliases = make(map[string]string)
	}
	m.userAliases[alias] = target
	_ = core.SaveUserAliases(m.userAliases)
	m.aliases.ByFullCommand = core.BuildUserAliasIndex(m.userAliases)
	m.corpus = core.NewCorpus(m.history, m.aliases)
	m.refreshCommands()
	return true
}

func (m *model) closeAlias() {
	m.aliasing = false
	m.aliasError = ""
	m.aliasInput.Blur()
}

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.cursor--
		m.clampCursor()
	case tea.KeyEnter:
		return m.run(string(m.editBuffer))
	case tea.KeyBackspace:
		if m.cursor > 0 {
			m.editBuffer = append(m.editBuffer[:m.cursor-1], m.editBuffer[m.cursor:]...)
			m.cursor--
		}
	case tea.KeyLeft:
		m.cursor--
		m.clampCursor()
	case tea.KeyRight:
		m.cursor++
		m.clampCursor()
	case tea.KeySpace:
		m.insertRunes([]rune{' '})
	case tea.KeyRunes:
		m.insertRunes(msg.Runes)
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.pending != "" {
		return m.applyOperator(key), nil
	}

	// Resolve a pending `g` (the first key of a `gg` sequence). `gg` jumps to
	// the top, or to the count-th line (1-indexed) when a count was given.
	if m.gPending {
		m.gPending = false
		if key == "g" {
			target := 0
			if m.count != "" {
				target = m.takeCount() - 1
			}
			m.setSelection(target)
		}
		m.count = ""
		return m, nil
	}

	// A leading non-zero digit (or any digit once a count is in progress)
	// builds up a numeric count for the next motion, like in vim.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && m.count != "") {
		m.count += key
		return m, nil
	}

	if key == "g" {
		m.gPending = true
		return m, nil
	}

	hasCount := m.count != ""
	count := m.takeCount()

	switch key {
	case "esc":
		m.leaveResults()
	case "G":
		// Jump to the last line, or to the count-th line when given.
		target := len(m.commands) - 1
		if hasCount {
			target = count - 1
		}
		m.setSelection(target)
	case "enter":
		return m.run(string(m.editBuffer))
	case "j", "ctrl+j", "ctrl+n":
		m.setSelection(m.selectedIndex + count)
	case "k", "ctrl+k", "ctrl+p":
		m.setSelection(m.selectedIndex - count)
	case "ctrl+d":
		m.setSelection(m.selectedIndex + maxResults)
	case "ctrl+u":
		m.setSelection(m.selectedIndex - maxResults)
	case "h":
		m.cursor -= count
		m.clampCursor()
	case "l":
		m.cursor += count
		m.clampCursor()
	case "0":
		m.cursor = 0
	case "$":
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
	case "w":
		for n := 0; n < count; n++ {
			m.cursor = nextWordStart(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "b":
		for n := 0; n < count; n++ {
			m.cursor = prevWordStart(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "i":
		m.mode = modeInsert
	case "a":
		m.mode = modeInsert
		m.cursor++
		if m.cursor > len(m.editBuffer) {
			m.cursor = len(m.editBuffer)
		}
	case "A":
		m.mode = modeInsert
		m.cursor = len(m.editBuffer)
	case "I":
		m.mode = modeInsert
		m.cursor = 0
	case "x":
		end := m.cursor + count
		if end > len(m.editBuffer) {
			end = len(m.editBuffer)
		}
		if len(m.editBuffer) > 0 && m.cursor < len(m.editBuffer) {
			m.editBuffer = append(m.editBuffer[:m.cursor], m.editBuffer[end:]...)
			m.clampCursor()
		}
	case "m":
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
			m.aliasTarget = m.commands[m.selectedIndex]
			m.aliasInput.SetValue("")
			m.aliasError = ""
			m.aliasing = true
			return m, m.aliasInput.Focus()
		}
	case "v":
		m.mode = modeVisual
		m.visualAnchor = m.cursor
	case "V":
		m.mode = modeVisual
		m.visualAnchor = 0
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
	case "d", "c", "y":
		m.pending = key
		m.pendingCount = count
	case "u":
		m.undoDelete()
	case "p":
		m.paste(true)
	case "P":
		m.paste(false)
	}
	return m, nil
}

// setSelection moves the highlighted row to i, clamped to the list bounds,
// and keeps it visible while reloading the edit buffer for the new command.
func (m *model) setSelection(i int) {
	if len(m.commands) == 0 {
		return
	}
	m.selectedIndex = max(0, min(i, len(m.commands)-1))
	m.ensureVisible()
	m.loadEditBuffer()
}

// takeCount consumes the pending numeric count, returning at least 1.
func (m *model) takeCount() int {
	n := 1
	if m.count != "" {
		if v, err := strconv.Atoi(m.count); err == nil && v > 0 {
			n = v
		}
		m.count = ""
	}
	return n
}

func (m model) applyOperator(key string) model {
	op := m.pending
	count := m.pendingCount
	if count < 1 {
		count = 1
	}
	m.pending = ""
	m.pendingCount = 0

	if op == "d" && key == "d" {
		m.deleteSelected()
		return m
	}

	start, end := m.cursor, m.cursor
	switch key {
	case op:
		start, end = 0, len(m.editBuffer)
	case "w":
		end = m.cursor
		for n := 0; n < count; n++ {
			end = nextWordStart(m.editBuffer, end)
		}
	case "$":
		end = len(m.editBuffer)
	case "0":
		start = 0
	case "b":
		start = m.cursor
		for n := 0; n < count; n++ {
			start = prevWordStart(m.editBuffer, start)
		}
	default:
		return m
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

func (m *model) deleteSelected() {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.commands) {
		return
	}
	cmd := m.commands[m.selectedIndex]
	key := core.NormalizeCommandKey(cmd)

	if m.deleted == nil {
		m.deleted = make(map[string]string)
	}
	m.deleted[key] = cmd
	m.undoStack = append(m.undoStack, key)

	m.persistDeleted()
	m.refreshCommands()
	m.loadEditBuffer()
}

func (m *model) undoDelete() {
	if len(m.undoStack) == 0 {
		return
	}
	key := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	delete(m.deleted, key)

	m.persistDeleted()
	m.refreshCommands()
	m.loadEditBuffer()
}

func (m *model) persistDeleted() {
	commands := make([]string, 0, len(m.deleted))
	for _, cmd := range m.deleted {
		commands = append(commands, cmd)
	}
	_ = core.SaveDeletedCommands(commands)
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
	rs := []rune(strings.ReplaceAll(text, "\n", " "))

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
	if m.mode != modeInsert {
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

package main

import (
	"fmt"
	"memcommands/core"
	"strconv"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func (m model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.requestQuit()
	}
	if msg.String() == "ctrl+a" {
		m.toggleAliasFilter()
		return m, nil
	}
	switch m.mode {
	case modeInsert:
		return m.updateInsert(msg)
	case modeVisual:
		return m.updateVisual(msg)
	}
	return m.updateNormal(msg)
}

// visualRange returns the half-open [start, end) rune span selected in visual
// mode, inclusive of the character under the cursor.
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

	// Resolve an armed `r`: overwrite the whole selection with the typed char.
	if m.rPending {
		return m.applyVisualReplace(key), nil
	}

	// Resolve an armed f/F/t/T: extend the selection to the target char.
	if m.findPending != "" {
		m.cursor = m.applyFind(key)
		m.clampCursor()
		return m, nil
	}

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
	case "h", "left":
		m.cursor -= count
		m.clampCursor()
	case "l", "right":
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
	case "e":
		for n := 0; n < count; n++ {
			m.cursor = wordEnd(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "f", "F", "t", "T":
		m.findPending = key
		m.findCount = count
	case ";":
		m.cursor = m.repeatFind(m.lastFindCmd, count)
		m.clampCursor()
	case ",":
		m.cursor = m.repeatFind(reverseFind(m.lastFindCmd), count)
		m.clampCursor()
	case "r":
		m.rPending = true
	case "~":
		start, end := m.visualRange()
		for i := start; i < end; i++ {
			m.editBuffer[i] = toggleCase(m.editBuffer[i])
		}
		m.cursor = start
		m.mode = modeNormal
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

// updateAlias edits the alias label in the vim buffer; enter commits from any mode.
func (m model) updateAlias(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m.requestQuit()
	}
	if msg.Type == tea.KeyEnter {
		if m.saveAlias() {
			m.closeAlias()
		}
		return m, nil
	}

	m.aliasError = ""
	switch m.mode {
	case modeInsert:
		return m.updateInsert(msg)
	case modeVisual:
		return m.updateVisual(msg)
	}
	return m.updateAliasNormal(msg)
}

// updateAliasNormal runs the shared buffer motions, with esc to cancel.
func (m model) updateAliasNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pending != "" {
		return m.applyOperator(key), nil
	}
	if m.rPending {
		return m.applyReplace(key), nil
	}
	if m.findPending != "" {
		m.cursor = m.applyFind(key)
		m.clampCursor()
		return m, nil
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && m.count != "") {
		m.count += key
		return m, nil
	}

	count := m.takeCount()
	if key == "esc" {
		m.closeAlias()
		return m, nil
	}
	m.editMotion(key, count)
	return m, nil
}

// beginAlias opens the alias editor, pre-filling any existing label; blank opens
// in insert mode to type, an existing one in normal mode for motions.
func (m *model) beginAlias() {
	command := m.commands[m.selectedIndex]
	m.commitEdit() // stage any pending command edit before reusing the buffer
	m.aliasTarget = command
	m.aliasError = ""

	existing := ""
	if labels := core.AliasesForCommand(command, m.aliases); len(labels) > 0 {
		existing = labels[0]
	}
	m.editBuffer = []rune(existing)
	m.cursor = 0
	m.aliasing = true
	if existing == "" {
		m.mode = modeInsert
	} else {
		m.mode = modeNormal
	}
	m.clampCursor()
}

// saveAlias binds the edited label to the target (one per command); blank clears it.
func (m *model) saveAlias() bool {
	alias := strings.TrimSpace(string(m.editBuffer))
	target := strings.Join(strings.Fields(strings.TrimSpace(m.aliasTarget)), " ")
	if target == "" {
		return false
	}

	if alias == "" {
		if m.clearAliasFor(m.aliasTarget) {
			m.dirty = true
			m.rebuildAliasIndex()
		}
		return true
	}

	// Reject a label already bound to a different command.
	if existing, ok := m.userAliases[alias]; ok &&
		core.NormalizeCommandKey(existing) != core.NormalizeCommandKey(target) {
		m.aliasError = fmt.Sprintf("alias %q already in use", alias)
		return false
	}

	// One alias per command: drop the old label before binding the new one.
	m.clearAliasFor(m.aliasTarget)
	if m.userAliases == nil {
		m.userAliases = make(map[string]string)
	}
	m.userAliases[alias] = target
	m.dirty = true
	m.rebuildAliasIndex()
	return true
}

// clearAliasFor drops any alias pointing at command, reporting if one was removed.
func (m *model) clearAliasFor(command string) bool {
	removed := false
	for _, label := range core.AliasesForCommand(command, m.aliases) {
		delete(m.userAliases, label)
		removed = true
	}
	return removed
}

// rebuildAliasIndex re-derives the alias index and result list from userAliases.
func (m *model) rebuildAliasIndex() {
	m.aliases.ByFullCommand = core.BuildUserAliasIndex(m.userAliases)
	m.corpus = core.NewCorpus(m.history, m.aliases)
	m.refreshCommands()
}

// updateCommand drives the ":" line: enter executes, esc/backspace-past-start
// cancels, else append.
func (m model) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.requestQuit()
	case "esc":
		m.commandMode = false
		m.commandLine = ""
		return m, nil
	case "enter":
		return m.runExCommand()
	case "backspace":
		if m.commandLine == "" {
			m.commandMode = false
			return m, nil
		}
		r := []rune(m.commandLine)
		m.commandLine = string(r[:len(r)-1])
		return m, nil
	}

	switch msg.Type {
	case tea.KeySpace:
		m.commandLine += " "
	case tea.KeyRunes:
		m.commandLine += string(msg.Runes)
	}
	return m, nil
}

// runExCommand interprets the typed ":" command; unknown ones report an error.
func (m model) runExCommand() (tea.Model, tea.Cmd) {
	cmd := strings.TrimSpace(m.commandLine)
	m.commandMode = false
	m.commandLine = ""

	switch cmd {
	case "w":
		m.save()
		m.statusMsg = "written"
		return m, nil
	case "wq", "x":
		m.save()
		return m.quit()
	case "q":
		if m.dirty {
			m.statusMsg = "unsaved changes — :w to save or :q! to discard"
			return m, nil
		}
		return m.quit()
	case "q!":
		return m.quit()
	default:
		m.statusMsg = fmt.Sprintf("not a command: :%s", cmd)
		return m, nil
	}
}

// save flushes staged edits, deletions, and aliases to disk.
func (m *model) save() {
	m.commitEdit()
	if !m.dirty {
		return
	}
	m.persistDeleted()
	_ = core.SaveEditedCommands(m.edited)
	_ = core.SaveUserAliases(m.userAliases)
	m.dirty = false
}

func (m *model) closeAlias() {
	m.aliasing = false
	m.aliasError = ""
	m.mode = modeNormal
	m.loadEditBuffer() // restore the command's edit buffer
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
	m.statusMsg = "" // any normal-mode key clears a transient message

	if m.pending != "" {
		return m.applyOperator(key), nil
	}

	// Resolve a pending `r`: the next key overwrites the char(s) under the cursor.
	if m.rPending {
		return m.applyReplace(key), nil
	}

	// Resolve a pending f/F/t/T: the next key is the target char to jump to.
	if m.findPending != "" {
		m.cursor = m.applyFind(key)
		m.clampCursor()
		return m, nil
	}

	// Resolve a pending `g`: `gg` jumps to the top, or to the count-th line
	// (1-indexed) when a count was given.
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

	// Digits build a numeric count for the next motion, like vim. A leading 0 is
	// the "0" motion, not a count.
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
	case "?":
		m.showHelp = true
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
	case "j", "ctrl+j", "ctrl+n", "down":
		m.setSelection(m.selectedIndex + count)
	case "k", "ctrl+k", "ctrl+p", "up":
		// Moving up past the first row returns to the search bar.
		if m.selectedIndex == 0 {
			m.leaveResults()
			return m, nil
		}
		m.setSelection(m.selectedIndex - count)
	case "ctrl+d":
		m.setSelection(m.selectedIndex + maxResults)
	case "ctrl+u":
		if m.selectedIndex == 0 {
			m.leaveResults()
			return m, nil
		}
		m.setSelection(m.selectedIndex - maxResults)
	case "m":
		if m.selectedIndex >= 0 && m.selectedIndex < len(m.commands) {
			m.beginAlias()
		}
	case ":":
		m.commitEdit() // stage any pending edit so :w/:q see it
		m.commandMode = true
		m.commandLine = ""
		m.statusMsg = ""
	case "u":
		m.undoDelete()
	default:
		m.editMotion(key, count)
	}
	return m, nil
}

// editMotion applies the normal-mode keys that edit the buffer, shared by the
// command and alias editors.
func (m *model) editMotion(key string, count int) {
	switch key {
	case "h", "left":
		m.cursor -= count
		m.clampCursor()
	case "l", "right":
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
	case "e":
		for n := 0; n < count; n++ {
			m.cursor = wordEnd(m.editBuffer, m.cursor)
		}
		m.clampCursor()
	case "f", "F", "t", "T":
		m.findPending = key
		m.findCount = count
	case ";":
		m.cursor = m.repeatFind(m.lastFindCmd, count)
		m.clampCursor()
	case ",":
		m.cursor = m.repeatFind(reverseFind(m.lastFindCmd), count)
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
	case "X":
		start := m.cursor - count
		if start < 0 {
			start = 0
		}
		if m.cursor > 0 {
			m.editBuffer = append(m.editBuffer[:start], m.editBuffer[m.cursor:]...)
			m.cursor = start
			m.clampCursor()
		}
	case "D":
		if m.cursor < len(m.editBuffer) {
			m.editBuffer = m.editBuffer[:m.cursor]
			m.clampCursor()
		}
	case "C":
		m.editBuffer = m.editBuffer[:m.cursor]
		m.mode = modeInsert
	case "s":
		end := m.cursor + count
		if end > len(m.editBuffer) {
			end = len(m.editBuffer)
		}
		if m.cursor < len(m.editBuffer) {
			m.editBuffer = append(m.editBuffer[:m.cursor], m.editBuffer[end:]...)
		}
		m.mode = modeInsert
	case "S":
		m.editBuffer = m.editBuffer[:0]
		m.cursor = 0
		m.mode = modeInsert
	case "Y":
		writeClipboard(string(m.editBuffer))
	case "~":
		for n := 0; n < count && m.cursor < len(m.editBuffer); n++ {
			m.editBuffer[m.cursor] = toggleCase(m.editBuffer[m.cursor])
			m.cursor++
		}
		m.clampCursor()
	case "v":
		m.mode = modeVisual
		m.visualAnchor = m.cursor
	case "V":
		m.mode = modeVisual
		m.visualAnchor = 0
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
	case "r":
		m.rPending = true
		m.pendingCount = count
	case "d", "c", "y":
		m.pending = key
		m.pendingCount = count
	case "p":
		m.paste(true)
	case "P":
		m.paste(false)
	}
}

// setSelection moves the highlighted row to i (clamped), keeps it visible, and
// reloads the edit buffer for the new command.
func (m *model) setSelection(i int) {
	if len(m.commands) == 0 {
		return
	}
	m.commitEdit()
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
		if m.aliasing {
			// `dd` on the single-line alias buffer clears it, like vim.
			m.editBuffer = m.editBuffer[:0]
			m.cursor = 0
			return m
		}
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
	case "e":
		end = m.cursor
		for n := 0; n < count; n++ {
			end = wordEnd(m.editBuffer, end)
		}
		end++ // operators are inclusive of the word-end char
		if end > len(m.editBuffer) {
			end = len(m.editBuffer)
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

// applyReplace overwrites count chars under the cursor with the typed key, no-op if too few remain or the key isn't printable.
func (m model) applyReplace(key string) model {
	count := m.pendingCount
	if count < 1 {
		count = 1
	}
	m.rPending = false
	m.pendingCount = 0

	rs := []rune(key)
	if len(rs) != 1 || m.cursor+count > len(m.editBuffer) {
		return m
	}
	for i := 0; i < count; i++ {
		m.editBuffer[m.cursor+i] = rs[0]
	}
	m.cursor += count - 1
	m.clampCursor()
	return m
}

// applyVisualReplace overwrites the selection with the typed key and returns to normal; a non-printable key cancels.
func (m model) applyVisualReplace(key string) model {
	m.rPending = false
	rs := []rune(key)
	if len(rs) != 1 {
		return m
	}
	start, end := m.visualRange()
	for i := start; i < end; i++ {
		m.editBuffer[i] = rs[0]
	}
	m.cursor = start
	m.mode = modeNormal
	m.clampCursor()
	return m
}

// applyFind resolves an armed f/F/t/T against the typed key, remembers it for ;/,, and returns the destination cursor.
func (m *model) applyFind(key string) int {
	cmd := m.findPending
	count := m.findCount
	if count < 1 {
		count = 1
	}
	m.findPending = ""
	m.findCount = 0

	rs := []rune(key)
	if len(rs) != 1 {
		return m.cursor
	}
	m.lastFindCmd = cmd
	m.lastFindChar = rs[0]
	return findDest(m.editBuffer, m.cursor, cmd, rs[0], count)
}

// repeatFind re-runs the last f/F/t/T (or its reverse for `,`) count times.
func (m model) repeatFind(cmd string, count int) int {
	if cmd == "" || m.lastFindCmd == "" {
		return m.cursor
	}
	return findDest(m.editBuffer, m.cursor, cmd, m.lastFindChar, count)
}

// reverseFind flips a find command's direction for the `,` repeat.
func reverseFind(cmd string) string {
	switch cmd {
	case "f":
		return "F"
	case "F":
		return "f"
	case "t":
		return "T"
	case "T":
		return "t"
	}
	return ""
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
	m.dirty = true

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
	m.dirty = true

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

// wordEnd returns the index of the current word's last char, or the next word's when already at a word end.
func wordEnd(rs []rune, i int) int {
	n := len(rs)
	if n == 0 {
		return 0
	}
	if i >= n-1 {
		return n - 1
	}
	i++
	for i < n && isSpace(rs[i]) {
		i++
	}
	for i+1 < n && !isSpace(rs[i+1]) {
		i++
	}
	if i >= n {
		i = n - 1
	}
	return i
}

// findDest computes the destination cursor for an f/F/t/T search of target repeated count times, or the original index if not found.
func findDest(rs []rune, from int, cmd string, target rune, count int) int {
	switch cmd {
	case "f":
		return findForward(rs, from, target, count)
	case "F":
		return findBackward(rs, from, target, count)
	case "t":
		if dest := findForward(rs, from, target, count); dest > from {
			return dest - 1
		}
	case "T":
		if dest := findBackward(rs, from, target, count); dest < from {
			return dest + 1
		}
	}
	return from
}

func findForward(rs []rune, from int, target rune, count int) int {
	pos := from
	for n := 0; n < count; n++ {
		i := pos + 1
		for i < len(rs) && rs[i] != target {
			i++
		}
		if i >= len(rs) {
			return from
		}
		pos = i
	}
	return pos
}

func findBackward(rs []rune, from int, target rune, count int) int {
	pos := from
	for n := 0; n < count; n++ {
		i := pos - 1
		for i >= 0 && rs[i] != target {
			i--
		}
		if i < 0 {
			return from
		}
		pos = i
	}
	return pos
}

func toggleCase(r rune) rune {
	switch {
	case unicode.IsUpper(r):
		return unicode.ToLower(r)
	case unicode.IsLower(r):
		return unicode.ToUpper(r)
	}
	return r
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

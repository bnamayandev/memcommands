package main

import (
	"fmt"
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

	// A leading non-zero digit (or any digit once a count is in progress)
	// builds up a numeric count for the next motion, like in vim.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' || (key == "0" && m.count != "") {
		m.count += key
		return m, nil
	}

	count := m.takeCount()

	switch key {
	case "esc":
		m.leaveResults()
	case "enter":
		return m.run(string(m.editBuffer))
	case "j", "ctrl+j", "ctrl+n":
		m.selectedIndex = max(0, min(m.selectedIndex+count, len(m.commands)-1))
		m.ensureVisible()
		m.loadEditBuffer()
	case "k", "ctrl+k", "ctrl+p":
		m.selectedIndex = max(m.selectedIndex-count, 0)
		m.ensureVisible()
		m.loadEditBuffer()
	case "ctrl+d":
		m.selectedIndex = min(m.selectedIndex+maxResults, len(m.commands)-1)
		m.ensureVisible()
		m.loadEditBuffer()
	case "ctrl+u":
		m.selectedIndex = max(m.selectedIndex-maxResults, 0)
		m.ensureVisible()
		m.loadEditBuffer()
	case "h":
		m.cursor--
		m.clampCursor()
	case "l":
		m.cursor++
		m.clampCursor()
	case "0":
		m.cursor = 0
	case "$":
		m.cursor = len(m.editBuffer) - 1
		m.clampCursor()
	case "w":
		m.cursor = nextWordStart(m.editBuffer, m.cursor)
		m.clampCursor()
	case "b":
		m.cursor = prevWordStart(m.editBuffer, m.cursor)
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
		if len(m.editBuffer) > 0 && m.cursor < len(m.editBuffer) {
			m.editBuffer = append(m.editBuffer[:m.cursor], m.editBuffer[m.cursor+1:]...)
			m.clampCursor()
		}
	case "d", "c", "y":
		m.pending = key
	case "p":
		m.paste(true)
	case "P":
		m.paste(false)
	}
	return m, nil
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
	m.pending = ""

	start, end := m.cursor, m.cursor
	switch key {
	case op:
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

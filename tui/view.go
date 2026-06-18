package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	return &Styles{
		FocusedBorder: lipgloss.Color("#89B4FA"),
		BlurredBorder: lipgloss.Color("#45475A"),
		userInput:     lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()),
		results:       lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()),
		selectedRow:   lipgloss.NewStyle().Background(lipgloss.Color("#313244")).Bold(true),
		cursor:        lipgloss.NewStyle().Reverse(true),
		status:        lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")),
	}
}

func (m model) View() string {
	contentWidth := m.contentWidth()

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
		Render(m.renderIndexedCommands(contentWidth))

	status := ansi.Truncate(m.statusLine(), m.width, "…")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		searchBlock,
		resultsBlock,
		m.styles.status.Render(status),
	)
}

func (m model) renderIndexedCommands(width int) string {
	start := m.scrollOffset
	end := min(start+maxResults, len(m.commands))
	lines := make([]string, 0, end-start)

	for i := start; i < end; i++ {
		selected := i == m.selectedIndex
		editing := selected && m.focus == focusResults

		text := m.commands[i]
		if editing {
			text = m.renderEditLine()
		}

		prefix := "  "
		if selected {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%d. %s", prefix, i+1, text)

		// Keep each row within the terminal width so long commands
		// truncate instead of wrapping and breaking the layout.
		if width > 0 {
			line = ansi.Truncate(line, width, "…")
		}

		if editing {
			line = m.styles.selectedRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderEditLine() string {
	var b strings.Builder
	for i, r := range m.editBuffer {
		if i == m.cursor {
			b.WriteString(m.styles.cursor.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	if m.cursor >= len(m.editBuffer) {
		b.WriteString(m.styles.cursor.Render(" "))
	}
	return b.String()
}

func (m model) statusLine() string {
	switch {
	case m.focus == focusSearch:
		return "SEARCH  enter: run top result   ctrl+j/n: focus results"
	case m.mode == modeInsert:
		return "-- INSERT --  esc: normal   enter: run"
	default:
		return "NORMAL  j/k: move  ctrl+d/u: page  h/l 0 $ w b: cursor  i/a: edit  x dd dw cw: delete  yy/y$: yank  p: paste  enter: run  esc: search"
	}
}

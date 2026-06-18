package main

import (
	"fmt"
	"memcommands/core"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Catppuccin Mocha palette.
const (
	colBase     = "#1E1E2E"
	colSurface0 = "#313244"
	colSurface1 = "#45475A"
	colOverlay0 = "#6C7086"
	colText     = "#CDD6F4"
	colBlue     = "#89B4FA"
	colGreen    = "#A6E3A1"
	colMauve    = "#CBA6F7"
)

// hPad is the horizontal padding inside each bordered block.
const hPad = 1

// aliasBoxWidth is the target width of the floating alias box.
const aliasBoxWidth = 50

type Styles struct {
	FocusedBorder lipgloss.Color
	BlurredBorder lipgloss.Color
	block         lipgloss.Style
	index         lipgloss.Style
	selectedRow   lipgloss.Style
	cursor        lipgloss.Style
	cursorBar     lipgloss.Style
	alias         lipgloss.Style
	hints         lipgloss.Style
	modeSearch    lipgloss.Style
	modeNormal    lipgloss.Style
	modeInsert    lipgloss.Style
}

func DefaultStyles() *Styles {
	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colBase)).
		Bold(true).
		Padding(0, 1)

	return &Styles{
		FocusedBorder: lipgloss.Color(colBlue),
		BlurredBorder: lipgloss.Color(colSurface1),
		block: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, hPad),
		index: lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)),
		selectedRow: lipgloss.NewStyle().
			Background(lipgloss.Color(colSurface0)).
			Foreground(lipgloss.Color(colText)).
			Bold(true),
		cursor:     lipgloss.NewStyle().Reverse(true),
		cursorBar:  lipgloss.NewStyle().Underline(true),
		alias:      lipgloss.NewStyle().Foreground(lipgloss.Color(colGreen)).Italic(true),
		hints:      lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)),
		modeSearch: badge.Background(lipgloss.Color(colBlue)),
		modeNormal: badge.Background(lipgloss.Color(colGreen)),
		modeInsert: badge.Background(lipgloss.Color(colMauve)),
	}
}

func (m model) View() string {
	if m.aliasing {
		return m.aliasView()
	}

	contentWidth := m.contentWidth()
	innerWidth := m.innerWidth()

	searchBorder := m.styles.BlurredBorder
	resultsBorder := m.styles.BlurredBorder
	if m.focus == focusSearch {
		searchBorder = m.styles.FocusedBorder
	} else {
		resultsBorder = m.styles.FocusedBorder
	}

	searchBlock := m.styles.block.
		BorderForeground(searchBorder).
		Width(contentWidth).
		Render(m.userInput.View())

	resultsBlock := m.styles.block.
		BorderForeground(resultsBorder).
		Width(contentWidth).
		Render(m.renderIndexedCommands(innerWidth))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		searchBlock,
		resultsBlock,
		m.statusBar(),
	)
}

// aliasView renders the floating alias box centered over the terminal.
func (m model) aliasView() string {
	boxWidth := min(aliasBoxWidth, max(0, m.width-4))
	inner := max(0, boxWidth-2*hPad)

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colMauve)).
		Bold(true).
		Render("set alias")

	target := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colOverlay0)).
		Render(ansi.Truncate(m.aliasTarget, inner, "…"))

	hints := m.styles.hints.Render("enter save · esc cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		target,
		"",
		m.aliasInput.View(),
		"",
		hints,
	)

	box := m.styles.block.
		BorderForeground(lipgloss.Color(colMauve)).
		Width(boxWidth).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderIndexedCommands(width int) string {
	if len(m.commands) == 0 {
		return m.styles.index.Render("  no matching commands")
	}

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
		suffix := m.aliasSuffix(m.commands[i])

		if selected {
			// The whole row gets a single uniform style so the highlight
			// fills the block and there are no color gaps.
			line := fmt.Sprintf("❯ %2d %s%s", i+1, text, suffix)
			if width > 0 {
				line = ansi.Truncate(line, width, "…")
			}
			lines = append(lines, m.styles.selectedRow.Width(width).Render(line))
			continue
		}

		line := m.styles.index.Render(fmt.Sprintf("  %2d ", i+1)) + text + suffix
		if width > 0 {
			line = ansi.Truncate(line, width, "…")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// aliasSuffix renders the user-defined alias labels for a command as a
// green-italic ` <name>` tag, or an empty string when there are none.
func (m model) aliasSuffix(command string) string {
	labels := core.AliasesForCommand(command, m.aliases)
	if len(labels) == 0 {
		return ""
	}

	tags := make([]string, len(labels))
	for i, label := range labels {
		tags[i] = "<" + label + ">"
	}
	return " " + m.styles.alias.Render(strings.Join(tags, " "))
}

func (m model) renderEditLine() string {
	if m.mode == modeInsert {
		var b strings.Builder
		for i, r := range m.editBuffer {
			if i == m.cursor {
				b.WriteString(m.styles.cursorBar.Render(string(r)))
			} else {
				b.WriteRune(r)
			}
		}
		if m.cursor >= len(m.editBuffer) {
			b.WriteString(m.styles.cursorBar.Render(" "))
		}
		return b.String()
	}

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

func (m model) statusBar() string {
	var badge, hints string
	switch {
	case m.focus == focusSearch:
		badge = m.styles.modeSearch.Render("SEARCH")
		hints = "enter run · ctrl+j/n focus results"
	case m.mode == modeInsert:
		badge = m.styles.modeInsert.Render("INSERT")
		hints = "esc normal · enter run"
	default:
		badge = m.styles.modeNormal.Render("NORMAL")
		hints = "j/k move · h/l 0 $ w b cursor · i/a edit · x dd dw cw del · yy/y$ yank · p paste · m alias · enter run · esc search"
	}

	bar := badge + " " + m.styles.hints.Render(hints)
	return ansi.Truncate(bar, m.width, "…")
}

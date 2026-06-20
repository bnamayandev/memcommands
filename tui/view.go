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
	colPeach    = "#FAB387"
	colRed      = "#F38BA8"
)

// hPad is the horizontal padding inside each bordered block.
const hPad = 1

type Styles struct {
	FocusedBorder lipgloss.Color
	BlurredBorder lipgloss.Color
	block         lipgloss.Style
	index         lipgloss.Style
	selectedRow   lipgloss.Style
	cursor        lipgloss.Style
	cursorBar     lipgloss.Style
	visual        lipgloss.Style
	alias         lipgloss.Style
	match         lipgloss.Style
	hints         lipgloss.Style
	modeSearch    lipgloss.Style
	modeNormal    lipgloss.Style
	modeInsert    lipgloss.Style
	modeVisual    lipgloss.Style
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
		visual: lipgloss.NewStyle().
			Background(lipgloss.Color(colBlue)).
			Foreground(lipgloss.Color(colBase)),
		alias:      lipgloss.NewStyle().Foreground(lipgloss.Color(colGreen)).Italic(true),
		match:      lipgloss.NewStyle().Foreground(lipgloss.Color(colPeach)).Bold(true),
		hints:      lipgloss.NewStyle().Foreground(lipgloss.Color(colOverlay0)),
		modeSearch: badge.Background(lipgloss.Color(colBlue)),
		modeNormal: badge.Background(lipgloss.Color(colGreen)),
		modeInsert: badge.Background(lipgloss.Color(colMauve)),
		modeVisual: badge.Background(lipgloss.Color(colBlue)),
	}
}

func (m model) View() string {
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

func (m model) renderIndexedCommands(width int) string {
	if len(m.commands) == 0 {
		return m.styles.index.Render("  no matching commands")
	}

	query := m.userInput.Value()

	start := m.scrollOffset
	end := min(start+maxResults, len(m.commands))
	lines := make([]string, 0, end-start)

	for i := start; i < end; i++ {
		selected := i == m.selectedIndex
		aliasing := selected && m.aliasing
		editing := selected && m.focus == focusResults && !m.aliasing

		var text string
		if editing {
			text = m.renderEditLine()
		} else {
			resolved := m.resolve(m.commands[i])
			text = highlightMatch(resolved, core.MatchPositions(query, resolved), m.styles.match)
		}

		prefix := m.aliasPrefix(m.commands[i])
		if aliasing {
			prefix = m.aliasEditPrefix()
		}

		if selected {
			// The whole row gets a single uniform style so the highlight
			// fills the block and there are no color gaps.
			line := fmt.Sprintf("❯ %2d %s%s", i+1, prefix, text)
			if width > 0 {
				line = ansi.Truncate(line, width, "…")
			}
			lines = append(lines, m.styles.selectedRow.Width(width).Render(line))
			continue
		}

		line := m.styles.index.Render(fmt.Sprintf("  %2d ", i+1)) + prefix + text
		if width > 0 {
			line = ansi.Truncate(line, width, "…")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m model) aliasEditPrefix() string {
	return m.styles.alias.Render("<") + m.aliasInput.View() + m.styles.alias.Render(">") + " "
}

func (m model) aliasPrefix(command string) string {
	labels := core.AliasesForCommand(command, m.aliases)
	if len(labels) == 0 {
		return ""
	}

	tags := make([]string, len(labels))
	for i, label := range labels {
		tags[i] = "<" + label + ">"
	}
	return m.styles.alias.Render(strings.Join(tags, " ")) + " "
}

func highlightMatch(text string, positions []int, style lipgloss.Style) string {
	if len(positions) == 0 {
		return text
	}

	set := make(map[int]struct{}, len(positions))
	for _, p := range positions {
		set[p] = struct{}{}
	}

	var b strings.Builder
	for i, r := range text {
		if _, ok := set[i]; ok {
			b.WriteString(style.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m model) renderEditLine() string {
	if m.mode == modeVisual {
		start, end := m.visualRange()
		var b strings.Builder
		for i, r := range m.editBuffer {
			if i >= start && i < end {
				b.WriteString(m.styles.visual.Render(string(r)))
			} else {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

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
	if m.aliasing {
		badge := m.styles.modeInsert.Render("ALIAS")
		hints := m.styles.hints.Render("enter save · esc cancel")
		if m.aliasError != "" {
			hints = lipgloss.NewStyle().Foreground(lipgloss.Color(colRed)).Render(m.aliasError)
		}
		return ansi.Truncate(badge+" "+hints, m.width, "…")
	}

	// The ":" command line takes over the status bar while typing.
	if m.commandMode {
		badge := m.styles.modeNormal.Render("COMMAND")
		line := m.styles.cursorBar.Render(":" + m.commandLine + " ")
		return ansi.Truncate(badge+" "+line, m.width, "…")
	}

	var badge, hints string
	switch {
	case m.focus == focusSearch:
		badge = m.styles.modeSearch.Render("SEARCH")
		hints = "enter run · ctrl+j/n focus results"
	case m.mode == modeInsert:
		badge = m.styles.modeInsert.Render("INSERT")
		hints = "esc normal · enter run"
	case m.mode == modeVisual:
		badge = m.styles.modeVisual.Render("VISUAL")
		hints = "h/l 0 $ w b extend · y yank · d/x del · c change · esc cancel"
	default:
		badge = m.styles.modeNormal.Render("NORMAL")
		hints = "j/k move · i/a edit · dd remove · u undo · m alias · :w/:wq/:q save · enter run · esc search"
	}

	// A transient message or unsaved marker replaces the hints when present.
	switch {
	case m.statusMsg != "":
		hints = m.statusMsg
	case m.dirty:
		hints = "[+] unsaved · " + hints
	}

	bar := badge + " " + m.styles.hints.Render(hints)
	return ansi.Truncate(bar, m.width, "…")
}

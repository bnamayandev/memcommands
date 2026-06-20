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

		if selected {
			lines = append(lines, m.renderSelectedRow(i, query, editing, aliasing, width))
			continue
		}

		resolved := m.resolve(m.commands[i])
		text := highlightMatch(resolved, core.MatchPositions(query, resolved), lipgloss.NewStyle(), m.styles.match)
		line := m.styles.index.Render(fmt.Sprintf("  %2d ", i+1)) + m.aliasPrefix(m.commands[i]) + text
		if width > 0 {
			line = ansi.Truncate(line, width, "…")
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderSelectedRow(i int, query string, editing, aliasing bool, width int) string {
	base := m.styles.selectedRow

	var content string
	if editing {
		content = m.renderEditLine(base)
	} else {
		resolved := m.resolve(m.commands[i])
		content = highlightMatch(resolved, core.MatchPositions(query, resolved), base, base.Foreground(lipgloss.Color(colPeach)))
	}

	var prefix string
	switch {
	case aliasing:
		prefix = m.aliasEditPrefix()
	case m.aliasTagText(m.commands[i]) != "":
		prefix = base.Italic(true).Foreground(lipgloss.Color(colGreen)).Render(m.aliasTagText(m.commands[i])) + base.Render(" ")
	}

	row := base.Render(fmt.Sprintf("❯ %2d ", i+1)) + prefix + content
	if width > 0 {
		row = ansi.Truncate(row, width, "…")
		if pad := width - ansi.StringWidth(row); pad > 0 {
			row += base.Render(strings.Repeat(" ", pad))
		}
	}
	return row
}

func (m model) aliasEditPrefix() string {
	return m.styles.alias.Render("<") + m.aliasInput.View() + m.styles.alias.Render(">") + " "
}

func (m model) aliasPrefix(command string) string {
	tags := m.aliasTagText(command)
	if tags == "" {
		return ""
	}
	return m.styles.alias.Render(tags) + " "
}

func (m model) aliasTagText(command string) string {
	labels := core.AliasesForCommand(command, m.aliases)
	if len(labels) == 0 {
		return ""
	}
	tags := make([]string, len(labels))
	for i, label := range labels {
		tags[i] = "<" + label + ">"
	}
	return strings.Join(tags, " ")
}

func highlightMatch(text string, positions []int, base, match lipgloss.Style) string {
	if len(positions) == 0 {
		return base.Render(text)
	}

	set := make(map[int]struct{}, len(positions))
	for _, p := range positions {
		set[p] = struct{}{}
	}

	var b strings.Builder
	for i, r := range text {
		if _, ok := set[i]; ok {
			b.WriteString(match.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

func (m model) renderEditLine(base lipgloss.Style) string {
	if m.mode == modeVisual {
		start, end := m.visualRange()
		visual := base.Background(lipgloss.Color(colBlue)).Foreground(lipgloss.Color(colBase))
		var b strings.Builder
		for i, r := range m.editBuffer {
			if i >= start && i < end {
				b.WriteString(visual.Render(string(r)))
			} else {
				b.WriteString(base.Render(string(r)))
			}
		}
		return b.String()
	}

	cursor := base.Reverse(true)
	if m.mode == modeInsert {
		cursor = base.Underline(true)
	}

	var b strings.Builder
	for i, r := range m.editBuffer {
		if i == m.cursor {
			b.WriteString(cursor.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	if m.cursor >= len(m.editBuffer) {
		b.WriteString(cursor.Render(" "))
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

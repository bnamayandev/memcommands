package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestSelectedRowBackgroundIsContinuous(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	m := keys(newModel(t), "ctrl+j", "l", "l", "l")
	row := m.renderSelectedRow(m.selectedIndex, "", true, 40)

	const bg = "48;2;"
	for _, seg := range strings.Split(row, "\x1b[0m") {
		if ansi.StringWidth(seg) == 0 {
			continue
		}
		if !strings.Contains(seg, bg) {
			t.Fatalf("selected-row segment renders without a background: %q", seg)
		}
	}
}

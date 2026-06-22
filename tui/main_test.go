package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"memcommands/core"
)

func keys(m model, ss ...string) model {
	for _, s := range ss {
		var msg tea.KeyMsg
		switch s {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "ctrl+j":
			msg = tea.KeyMsg{Type: tea.KeyCtrlJ}
		case "ctrl+p":
			msg = tea.KeyMsg{Type: tea.KeyCtrlP}
		case "ctrl+u":
			msg = tea.KeyMsg{Type: tea.KeyCtrlU}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
		}
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

func newModel(t *testing.T) model {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
}

func TestEnterOnSearchRunsFirst(t *testing.T) {
	m := keys(newModel(t), "enter")
	if m.executed != "go build ./..." {
		t.Fatalf("want first command executed, got %q", m.executed)
	}
}

func TestCtrlJFocusesResults(t *testing.T) {
	m := keys(newModel(t), "ctrl+j")
	if m.focus != focusResults || m.mode != modeNormal {
		t.Fatalf("expected results/normal focus, got focus=%d mode=%d", m.focus, m.mode)
	}
	if string(m.editBuffer) != "go build ./..." {
		t.Fatalf("edit buffer not loaded: %q", string(m.editBuffer))
	}
}

func TestInsertEdit(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "A", "space", "-", "v", "enter")
	if m.executed != "go build ./... -v" {
		t.Fatalf("want edited command, got %q", m.executed)
	}
}

func TestDeleteCommandRemovesFromList(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "d", "d")
	if len(m.commands) != 1 || m.commands[0] != "git status" {
		t.Fatalf("want only 'git status' left, got %v", m.commands)
	}
	if string(m.editBuffer) != "git status" {
		t.Fatalf("want buffer to reload to 'git status', got %q", string(m.editBuffer))
	}
}

func TestUndoRestoresDeletedCommand(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "d", "d", "u")
	if len(m.commands) != 2 {
		t.Fatalf("want both commands restored, got %v", m.commands)
	}
}

func TestDeleteIsStagedUntilWritten(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Delete without saving: the change is staged, not yet on disk.
	keys(*New([]string{"git status", "go build ./..."}, core.AliasIndex{}), "ctrl+j", "d", "d")

	unsaved := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	if len(unsaved.commands) != 2 {
		t.Fatalf("staged deletion should not persist without :w, got %v", unsaved.commands)
	}
}

func TestDeletedCommandStaysGoneAfterWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// :w writes the staged deletion to disk.
	keys(*New([]string{"git status", "go build ./..."}, core.AliasIndex{}), "ctrl+j", "d", "d", ":", "w", "enter")

	reloaded := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	if len(reloaded.commands) != 1 || reloaded.commands[0] != "git status" {
		t.Fatalf("want deletion to persist after :w, got %v", reloaded.commands)
	}
}

func TestDirtyFlagSetAndCleared(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "d", "d")
	if !m.dirty {
		t.Fatalf("expected dirty after delete")
	}
	m = keys(m, ":", "w", "enter")
	if m.dirty {
		t.Fatalf("expected clean after :w")
	}
	if m.statusMsg != "written" {
		t.Fatalf("want 'written' status, got %q", m.statusMsg)
	}
}

func TestQuitWarnsWhenDirty(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "d", "d", ":", "q", "enter")
	if m.executed != "" {
		t.Fatalf("dirty :q should not run/quit")
	}
	if m.statusMsg == "" {
		t.Fatalf("dirty :q should warn in status bar")
	}
}

func TestForceQuitDiscardsStagedDeletion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	keys(*New([]string{"git status", "go build ./..."}, core.AliasIndex{}), "ctrl+j", "d", "d", ":", "q", "!", "enter")

	reloaded := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	if len(reloaded.commands) != 2 {
		t.Fatalf(":q! should discard staged deletion, got %v", reloaded.commands)
	}
}

func TestUndoPersistsThroughSession(t *testing.T) {
	// Delete two, navigate around, then undo both — all in one session.
	m := keys(newModel(t), "ctrl+j", "d", "d", "j", "d", "d", "j", "u", "u")
	if len(m.commands) != 2 {
		t.Fatalf("want both restored, got %v", m.commands)
	}
}

func TestUpAtFirstRowReturnsToSearch(t *testing.T) {
	// Pressing an up motion on row 1 falls back to the search bar, like esc.
	for _, key := range []string{"k", "ctrl+p", "ctrl+u"} {
		m := keys(newModel(t), "ctrl+j")
		if m.focus != focusResults {
			t.Fatalf("%s: expected to start in results", key)
		}
		m = keys(m, key)
		if m.focus != focusSearch {
			t.Fatalf("%s: expected up on row 1 to return to search, got focus %v", key, m.focus)
		}
	}
}

func TestBufferEditSurvivesNavigation(t *testing.T) {
	// Edit "go build ./..." down to "go build", move away, come back.
	m := keys(newModel(t), "ctrl+j", "A")
	for i := 0; i < len(" ./..."); i++ {
		m = keys(m, "backspace")
	}
	m = keys(m, "esc", "j", "k")
	if string(m.editBuffer) != "go build" {
		t.Fatalf("edit should survive navigation, got %q", string(m.editBuffer))
	}
}

func TestBufferEditPersistsAfterWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	m := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	m = keys(m, "ctrl+j", "A")
	for i := 0; i < len(" ./..."); i++ {
		m = keys(m, "backspace")
	}
	m = keys(m, "esc", ":", "w", "enter")

	reloaded := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	reloaded = keys(reloaded, "ctrl+j")
	if string(reloaded.editBuffer) != "go build" {
		t.Fatalf("edit should persist after :w, got %q", string(reloaded.editBuffer))
	}
}

func TestRevertingEditClearsOverlay(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "x", "j", "k")
	if string(m.editBuffer) != "o build ./..." {
		t.Fatalf("setup: want edited buffer, got %q", string(m.editBuffer))
	}
	// Put the deleted rune back; the overlay should drop the entry.
	m = keys(m, "I", "g", "esc")
	m = keys(m, "j", "k")
	if string(m.editBuffer) != "go build ./..." {
		t.Fatalf("reverting should restore original, got %q", string(m.editBuffer))
	}
	if _, ok := m.edited[core.NormalizeCommandKey("go build ./...")]; ok {
		t.Fatalf("overlay entry should be cleared after revert")
	}
}

func TestXDeletesChar(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "x")
	if string(m.editBuffer) != "o build ./..." {
		t.Fatalf("want 'o build ./...', got %q", string(m.editBuffer))
	}
}

func TestWordMotionThenDeleteWord(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "d", "w")
	if string(m.editBuffer) != "build ./..." {
		t.Fatalf("want 'build ./...', got %q", string(m.editBuffer))
	}
}

func TestCountMovesCursorRight(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "3", "l")
	if m.cursor != 3 {
		t.Fatalf("want cursor 3, got %d", m.cursor)
	}
}

func TestCountMovesCursorLeft(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "$", "2", "h")
	end := len("go build ./...") - 1
	if m.cursor != end-2 {
		t.Fatalf("want cursor %d, got %d", end-2, m.cursor)
	}
}

func TestCountDeletesChars(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "3", "x")
	if string(m.editBuffer) != "build ./..." {
		t.Fatalf("want 'build ./...', got %q", string(m.editBuffer))
	}
}

func TestCountWordMotion(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "2", "w")
	if m.cursor != 9 {
		t.Fatalf("want cursor 9 (start of './...'), got %d", m.cursor)
	}
}

func TestCountBackWordMotion(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "$", "2", "b")
	if m.cursor != 3 {
		t.Fatalf("want cursor 3 (start of 'build'), got %d", m.cursor)
	}
}

func TestCountDeleteWords(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "2", "d", "w")
	if string(m.editBuffer) != "./..." {
		t.Fatalf("want './...', got %q", string(m.editBuffer))
	}
}

func TestNavigateReloadsBuffer(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "x", "j")
	if string(m.editBuffer) != "git status" {
		t.Fatalf("buffer should reload on navigate, got %q", string(m.editBuffer))
	}
}

func TestEscFromNormalReturnsToSearch(t *testing.T) {
	m := keys(newModel(t), "ctrl+j", "esc")
	if m.focus != focusSearch {
		t.Fatalf("expected to return to search focus")
	}
}

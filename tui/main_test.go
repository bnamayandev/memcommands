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
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace}
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

func TestDeletedCommandStaysGoneOnReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	keys(*New([]string{"git status", "go build ./..."}, core.AliasIndex{}), "ctrl+j", "d", "d")

	reloaded := *New([]string{"git status", "go build ./..."}, core.AliasIndex{})
	if len(reloaded.commands) != 1 || reloaded.commands[0] != "git status" {
		t.Fatalf("want deletion to persist, got %v", reloaded.commands)
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

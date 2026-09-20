package main

import (
	"memcommands/core"
	"testing"
)

// newTestModel builds a model from a fixed session list without touching a
// real zellij install, isolating config I/O to a temp dir.
func newTestModel(t *testing.T, sessions []core.Session) *model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(sessions, core.AliasIndex{})
	m.focus = focusResults
	return m
}

// selectRow points selectedIndex at the row for name, failing the test if it's not present.
func selectRow(t *testing.T, m *model, name string) {
	t.Helper()
	for i, c := range m.commands {
		if c == name {
			m.selectedIndex = i
			m.loadEditBuffer()
			return
		}
	}
	t.Fatalf("session %q not found in commands %v", name, m.commands)
}

func TestArmKillSkipsExitedSessions(t *testing.T) {
	m := newTestModel(t, []core.Session{
		{Name: "running", Exited: false},
		{Name: "dead", Exited: true},
	})

	selectRow(t, m, "dead")
	got, _ := m.armKill(1)
	if got.confirm != confirmNone {
		t.Fatalf("expected no confirm armed for an already-exited session, got %v", got.confirm)
	}
	if got.statusMsg == "" {
		t.Fatalf("expected a status message explaining why nothing was armed")
	}

	selectRow(t, m, "running")
	got, _ = m.armKill(1)
	if got.confirm != confirmKill {
		t.Fatalf("expected confirmKill armed, got %v", got.confirm)
	}
	if len(got.confirmTargets) != 1 || got.confirmTargets[0] != "running" {
		t.Fatalf("expected [\"running\"] armed, got %v", got.confirmTargets)
	}
}

func TestArmDeleteWorksRegardlessOfSessionState(t *testing.T) {
	m := newTestModel(t, []core.Session{
		{Name: "running", Exited: false},
		{Name: "dead", Exited: true},
	})

	selectRow(t, m, "running")
	got, _ := m.armDelete(1)
	if got.confirm != confirmDelete || len(got.confirmTargets) != 1 || got.confirmTargets[0] != "running" {
		t.Fatalf("expected confirmDelete armed for \"running\", got confirm=%v targets=%v", got.confirm, got.confirmTargets)
	}

	selectRow(t, m, "dead")
	got, _ = m.armDelete(1)
	if got.confirm != confirmDelete || len(got.confirmTargets) != 1 || got.confirmTargets[0] != "dead" {
		t.Fatalf("expected confirmDelete armed for \"dead\", got confirm=%v targets=%v", got.confirm, got.confirmTargets)
	}
}

func TestArmDeleteRespectsCount(t *testing.T) {
	m := newTestModel(t, []core.Session{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	})
	// Select the first row directly, regardless of the corpus's tie-break
	// ordering, so the test doesn't depend on which name lands where.
	m.selectedIndex = 0
	m.loadEditBuffer()

	got, _ := m.armDelete(2)
	if got.confirm != confirmDelete || len(got.confirmTargets) != 2 {
		t.Fatalf("expected 2 targets armed, got confirm=%v targets=%v", got.confirm, got.confirmTargets)
	}
}

func TestCommitRenameBlocksExitedSession(t *testing.T) {
	m := newTestModel(t, []core.Session{
		{Name: "dead", Exited: true},
	})
	selectRow(t, m, "dead")

	// Simulate typing a new name over the exited session's row.
	m.editBuffer = []rune("renamed")
	m.commitRename()

	if m.statusMsg == "" {
		t.Fatalf("expected a status message blocking the rename")
	}
	if len(m.sessions) != 1 || m.sessions[0].Name != "dead" {
		t.Fatalf("expected the session list to be untouched, got %v", m.sessions)
	}
}

// TestEnterCommitsRenameBeforeAttaching guards against attaching to
// whatever uncommitted text is sitting in the edit buffer: it should attach
// to the session's real (committed) name instead. Exercised on an EXITED
// session, whose commitRename always reverts the buffer without shelling out
// to the real zellij binary, so this stays deterministic and offline.
func TestEnterCommitsRenameBeforeAttaching(t *testing.T) {
	m := newTestModel(t, []core.Session{
		{Name: "dead", Exited: true},
	})
	selectRow(t, m, "dead")

	// Simulate an in-progress, uncommitted edit of the row.
	m.editBuffer = []rune("typo-name")

	next, _ := m.attachEditedSelection()
	got := next.(model)
	if got.attachName != "dead" {
		t.Fatalf("expected attach to commit-reverted name \"dead\", got %q", got.attachName)
	}
}

func TestNewExCommandFallsBackToSearchText(t *testing.T) {
	m := newTestModel(t, nil)
	m.focus = focusSearch
	m.userInput.SetValue("my-new-session")
	m.commandLine = "new"

	next, cmd := m.runExCommand()
	got := next.(model)
	if got.attachName != "my-new-session" || !got.attachCreate {
		t.Fatalf("expected attach-create of the search text, got name=%q create=%v", got.attachName, got.attachCreate)
	}
	if cmd == nil {
		t.Fatalf("expected a quit command to be returned")
	}
}

func TestNewExCommandExplicitNameOverridesSearchText(t *testing.T) {
	m := newTestModel(t, nil)
	m.focus = focusSearch
	m.userInput.SetValue("ignored")
	m.commandLine = "new explicit-name"

	next, _ := m.runExCommand()
	got := next.(model)
	if got.attachName != "explicit-name" {
		t.Fatalf("expected explicit-name, got %q", got.attachName)
	}
}

func TestNewExCommandRequiresAName(t *testing.T) {
	m := newTestModel(t, nil)
	m.focus = focusSearch
	m.commandLine = "new"

	next, _ := m.runExCommand()
	got := next.(model)
	if got.attachName != "" {
		t.Fatalf("expected no attach without a name, got %q", got.attachName)
	}
	if got.statusMsg == "" {
		t.Fatalf("expected a usage message")
	}
}

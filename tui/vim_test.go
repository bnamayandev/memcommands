package main

import (
	"testing"

	"memcommands/core"
)

// The fixture model holds two commands. The corpus sorts them so that index 0
// is "go build ./..." (14 runes) and index 1 is "git status". Buffer indices
// for "go build ./...":
//
//	g0 o1 _2 b3 u4 i5 l6 d7 _8 .9 /10 .11 .12 .13
const (
	cmd0 = "go build ./..."
	cmd1 = "git status"
)

// enter results focus in normal mode, selection on cmd0.
func results(t *testing.T) model {
	return keys(newModel(t), "ctrl+j")
}

func wantBuf(t *testing.T, m model, want string) {
	t.Helper()
	if string(m.editBuffer) != want {
		t.Fatalf("buffer = %q, want %q", string(m.editBuffer), want)
	}
}

func wantCursor(t *testing.T, m model, want int) {
	t.Helper()
	if m.cursor != want {
		t.Fatalf("cursor = %d, want %d", m.cursor, want)
	}
}

// ---------------------------------------------------------------------------
// Normal-mode cursor motions
// ---------------------------------------------------------------------------

func TestMotionLRClampAtBounds(t *testing.T) {
	m := results(t)
	wantCursor(t, m, 0)
	m = keys(m, "h") // already at start
	wantCursor(t, m, 0)
	m = keys(m, "$")
	wantCursor(t, m, len(cmd0)-1)
	m = keys(m, "l") // already at last char (normal mode can't pass end)
	wantCursor(t, m, len(cmd0)-1)
}

func TestMotionZeroAndDollar(t *testing.T) {
	m := keys(results(t), "l", "l", "l")
	wantCursor(t, m, 3)
	m = keys(m, "0")
	wantCursor(t, m, 0)
	m = keys(m, "$")
	wantCursor(t, m, len(cmd0)-1)
}

func TestMotionWordForwardClampsAtEnd(t *testing.T) {
	m := keys(results(t), "w", "w", "w", "w")
	// Word starts are 0,3,9; further w clamps to last char.
	wantCursor(t, m, len(cmd0)-1)
}

func TestMotionWordBackToStart(t *testing.T) {
	m := keys(results(t), "$", "b", "b", "b")
	wantCursor(t, m, 0)
}

func TestCountResetsAfterMotion(t *testing.T) {
	m := keys(results(t), "3", "l")
	wantCursor(t, m, 3)
	// A bare l afterwards moves only one: the count was consumed.
	m = keys(m, "l")
	wantCursor(t, m, 4)
}

// ---------------------------------------------------------------------------
// Arrow keys mirror hjkl (normal/visual) and drop into results from search
// ---------------------------------------------------------------------------

func TestArrowsMoveCursorInNormal(t *testing.T) {
	m := keys(results(t), "right", "right", "right")
	wantCursor(t, m, 3)
	m = keys(m, "left")
	wantCursor(t, m, 2)
	// Counts apply just like l/h.
	m = keys(m, "2", "right")
	wantCursor(t, m, 4)
}

func TestArrowsNavigateList(t *testing.T) {
	m := keys(results(t), "down")
	if m.selectedIndex != 1 {
		t.Fatalf("down -> index %d, want 1", m.selectedIndex)
	}
	m = keys(m, "up")
	if m.selectedIndex != 0 {
		t.Fatalf("up -> index %d, want 0", m.selectedIndex)
	}
}

func TestUpArrowAtFirstRowReturnsToSearch(t *testing.T) {
	m := keys(results(t), "up")
	if m.focus != focusSearch {
		t.Fatalf("up on row 1 should return to search, got focus %v", m.focus)
	}
}

func TestDownArrowFromSearchEntersResults(t *testing.T) {
	m := keys(newModel(t), "down")
	if m.focus != focusResults || m.mode != modeNormal {
		t.Fatalf("down from search should enter results, got focus=%d mode=%d", m.focus, m.mode)
	}
}

func TestArrowsExtendVisualSelection(t *testing.T) {
	m := keys(results(t), "v", "right", "d") // select "go"
	wantBuf(t, m, " build ./...")
}

// ---------------------------------------------------------------------------
// List navigation
// ---------------------------------------------------------------------------

func TestNavDownUpAndClamp(t *testing.T) {
	m := keys(results(t), "j")
	if m.selectedIndex != 1 {
		t.Fatalf("j -> index %d, want 1", m.selectedIndex)
	}
	m = keys(m, "5", "j") // clamps at last
	if m.selectedIndex != 1 {
		t.Fatalf("count j past end -> %d, want 1", m.selectedIndex)
	}
	m = keys(m, "5", "k") // clamps at first
	if m.selectedIndex != 0 {
		t.Fatalf("count k past start -> %d, want 0", m.selectedIndex)
	}
}

func TestGotoTopBottom(t *testing.T) {
	m := keys(results(t), "G")
	if m.selectedIndex != 1 {
		t.Fatalf("G -> %d, want 1", m.selectedIndex)
	}
	m = keys(m, "g", "g")
	if m.selectedIndex != 0 {
		t.Fatalf("gg -> %d, want 0", m.selectedIndex)
	}
	m = keys(m, "2", "G")
	if m.selectedIndex != 1 {
		t.Fatalf("2G -> %d, want 1", m.selectedIndex)
	}
	m = keys(m, "1", "g", "g")
	if m.selectedIndex != 0 {
		t.Fatalf("1gg -> %d, want 0", m.selectedIndex)
	}
}

// ---------------------------------------------------------------------------
// Insert-mode entry points
// ---------------------------------------------------------------------------

func TestInsertAtCursor(t *testing.T) {
	m := keys(results(t), "i", "X")
	wantBuf(t, m, "Xgo build ./...")
}

func TestAppendAfterCursor(t *testing.T) {
	m := keys(results(t), "a", "X")
	wantBuf(t, m, "gXo build ./...")
}

func TestAppendAtEndOfLine(t *testing.T) {
	m := keys(results(t), "A", "X")
	wantBuf(t, m, "go build ./...X")
}

func TestInsertAtStartOfLine(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "I", "X")
	wantBuf(t, m, "Xgo build ./...")
	wantCursor(t, m, 1)
}

func TestInsertBackspaceAtStartIsNoop(t *testing.T) {
	m := keys(results(t), "i", "backspace")
	wantBuf(t, m, cmd0)
	wantCursor(t, m, 0)
}

func TestInsertBackspaceDeletesLeftOfCursor(t *testing.T) {
	m := keys(results(t), "A", "backspace")
	wantBuf(t, m, "go build ./..")
}

// ---------------------------------------------------------------------------
// x / delete-char
// ---------------------------------------------------------------------------

func TestDeleteCharCountClampsAtEnd(t *testing.T) {
	m := keys(results(t), "$", "3", "x") // only one char remains under/after cursor
	wantBuf(t, m, "go build ./..")
}

func TestDeleteCharOnEmptyBufferIsSafe(t *testing.T) {
	m := keys(results(t), "d", "$") // empties the buffer
	wantBuf(t, m, "")
	m = keys(m, "x") // must not panic or corrupt
	wantBuf(t, m, "")
}

// ---------------------------------------------------------------------------
// r / replace-char
// ---------------------------------------------------------------------------

func TestReplaceCharUnderCursor(t *testing.T) {
	m := keys(results(t), "r", "X")
	wantBuf(t, m, "Xo build ./...")
	wantCursor(t, m, 0) // cursor stays on the replaced char
}

func TestReplaceCharAtCursorPosition(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "r", "Z")
	wantBuf(t, m, "go Zuild ./...")
	wantCursor(t, m, 3)
}

func TestReplaceCountChars(t *testing.T) {
	m := keys(results(t), "3", "r", "X")
	wantBuf(t, m, "XXXbuild ./...") // overwrites "go " (incl. the space)
	wantCursor(t, m, 2)             // cursor lands on the last replaced char
}

func TestReplaceCountPastEndIsNoop(t *testing.T) {
	m := keys(results(t), "$", "3", "r", "X") // only one char left under cursor
	wantBuf(t, m, cmd0)
}

func TestReplaceWithSpace(t *testing.T) {
	m := keys(results(t), "r", " ")
	wantBuf(t, m, " o build ./...")
}

func TestReplaceEscCancels(t *testing.T) {
	m := keys(results(t), "r", "esc")
	wantBuf(t, m, cmd0)
	if m.rPending {
		t.Fatalf("pending r should clear after esc")
	}
	if m.mode != modeNormal {
		t.Fatalf("r esc should stay in normal mode, got %d", m.mode)
	}
}

// ---------------------------------------------------------------------------
// Operators: d / c / y
// ---------------------------------------------------------------------------

func TestDeleteToEndOfLine(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "d", "$")
	wantBuf(t, m, "go ")
}

func TestDeleteToStartOfLine(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "d", "0")
	wantBuf(t, m, "build ./...")
}

func TestChangeWordEntersInsert(t *testing.T) {
	m := keys(results(t), "c", "w")
	if m.mode != modeInsert {
		t.Fatalf("cw should enter insert mode, got %d", m.mode)
	}
	wantBuf(t, m, "build ./...")
	m = keys(m, "X")
	wantBuf(t, m, "Xbuild ./...")
}

func TestChangeWholeLine(t *testing.T) {
	m := keys(results(t), "c", "c")
	if m.mode != modeInsert {
		t.Fatalf("cc should enter insert mode, got %d", m.mode)
	}
	wantBuf(t, m, "")
}

func TestYankLeavesBufferUntouched(t *testing.T) {
	m := keys(results(t), "y", "w")
	wantBuf(t, m, cmd0)
	if m.mode != modeNormal {
		t.Fatalf("yw should stay in normal mode, got %d", m.mode)
	}
}

func TestOperatorWithBadMotionIsNoop(t *testing.T) {
	m := keys(results(t), "d", "z") // z is not a motion
	wantBuf(t, m, cmd0)
	if m.pending != "" {
		t.Fatalf("pending operator should clear after bad motion, got %q", m.pending)
	}
}

// ---------------------------------------------------------------------------
// e / word-end motion and operator
// ---------------------------------------------------------------------------

func TestWordEndMotion(t *testing.T) {
	m := keys(results(t), "e")
	wantCursor(t, m, 1) // end of "go"
	m = keys(m, "e")
	wantCursor(t, m, 7) // end of "build"
	m = keys(m, "e")
	wantCursor(t, m, len(cmd0)-1)
}

func TestDeleteToWordEnd(t *testing.T) {
	m := keys(results(t), "d", "e") // delete "go" inclusive
	wantBuf(t, m, " build ./...")
}

func TestChangeToWordEndEntersInsert(t *testing.T) {
	m := keys(results(t), "c", "e", "X")
	if m.mode != modeInsert {
		t.Fatalf("ce should enter insert, got %d", m.mode)
	}
	wantBuf(t, m, "X build ./...")
}

// ---------------------------------------------------------------------------
// f / F / t / T find-char and ; , repeat
// ---------------------------------------------------------------------------

func TestFindCharForward(t *testing.T) {
	m := keys(results(t), "f", "b")
	wantCursor(t, m, 3)
}

func TestFindCharNotFoundStaysPut(t *testing.T) {
	m := keys(results(t), "f", "z")
	wantCursor(t, m, 0)
}

func TestTillCharForward(t *testing.T) {
	m := keys(results(t), "t", "b")
	wantCursor(t, m, 2) // one before 'b'
}

func TestFindCharBackward(t *testing.T) {
	m := keys(results(t), "$", "F", "b")
	wantCursor(t, m, 3)
}

func TestFindRepeatAndReverse(t *testing.T) {
	m := keys(results(t), "f", ".") // first '.' at index 9
	wantCursor(t, m, 9)
	m = keys(m, ";") // next '.'
	wantCursor(t, m, 11)
	m = keys(m, ",") // reverse back
	wantCursor(t, m, 9)
}

func TestFindEscCancels(t *testing.T) {
	m := keys(results(t), "f", "esc")
	wantCursor(t, m, 0)
	if m.findPending != "" {
		t.Fatalf("pending find should clear after esc")
	}
}

// ---------------------------------------------------------------------------
// D / C / s / S / X / ~ / Y
// ---------------------------------------------------------------------------

func TestDeleteToEndShortcut(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "D")
	wantBuf(t, m, "go ")
}

func TestChangeToEndShortcut(t *testing.T) {
	m := keys(results(t), "l", "l", "l", "C", "X")
	if m.mode != modeInsert {
		t.Fatalf("C should enter insert, got %d", m.mode)
	}
	wantBuf(t, m, "go X")
}

func TestSubstituteChar(t *testing.T) {
	m := keys(results(t), "s", "Z")
	if m.mode != modeInsert {
		t.Fatalf("s should enter insert, got %d", m.mode)
	}
	wantBuf(t, m, "Zo build ./...")
}

func TestSubstituteLine(t *testing.T) {
	m := keys(results(t), "S", "Z")
	wantBuf(t, m, "Z")
}

func TestDeleteCharBeforeCursor(t *testing.T) {
	m := keys(results(t), "l", "l", "X")
	wantBuf(t, m, "g build ./...")
	wantCursor(t, m, 1)
}

func TestDeleteCharBeforeCursorAtStartIsNoop(t *testing.T) {
	m := keys(results(t), "X")
	wantBuf(t, m, cmd0)
}

func TestToggleCase(t *testing.T) {
	m := keys(results(t), "~")
	wantBuf(t, m, "Go build ./...")
	wantCursor(t, m, 1)
}

func TestToggleCaseCount(t *testing.T) {
	m := keys(results(t), "3", "~")
	wantBuf(t, m, "GO build ./...") // space is unaffected
	wantCursor(t, m, 3)
}

// ---------------------------------------------------------------------------
// Visual mode
// ---------------------------------------------------------------------------

func TestVisualDeleteSingleChar(t *testing.T) {
	m := keys(results(t), "v", "d")
	wantBuf(t, m, "o build ./...")
	if m.mode != modeNormal {
		t.Fatalf("visual d should return to normal, got %d", m.mode)
	}
}

func TestVisualDeleteRange(t *testing.T) {
	m := keys(results(t), "v", "l", "d") // select "go"
	wantBuf(t, m, " build ./...")
}

func TestVisualChangeEntersInsert(t *testing.T) {
	m := keys(results(t), "v", "l", "c")
	if m.mode != modeInsert {
		t.Fatalf("visual c should enter insert, got %d", m.mode)
	}
	wantBuf(t, m, " build ./...")
	m = keys(m, "X")
	wantBuf(t, m, "X build ./...")
}

func TestVisualLineSelectsWholeBuffer(t *testing.T) {
	m := keys(results(t), "V", "d")
	wantBuf(t, m, "")
}

func TestVisualYankReturnsToNormal(t *testing.T) {
	m := keys(results(t), "v", "l", "l", "y")
	wantBuf(t, m, cmd0)
	if m.mode != modeNormal {
		t.Fatalf("visual y should return to normal, got %d", m.mode)
	}
	wantCursor(t, m, 0) // cursor collapses to range start
}

func TestVisualReplaceFillsSelection(t *testing.T) {
	m := keys(results(t), "v", "l", "r", "X") // select "go", replace both
	wantBuf(t, m, "XX build ./...")
	if m.mode != modeNormal {
		t.Fatalf("visual r should return to normal, got %d", m.mode)
	}
}

func TestVisualReplaceEscStaysVisual(t *testing.T) {
	m := keys(results(t), "v", "l", "r", "esc")
	wantBuf(t, m, cmd0)
	if m.mode != modeVisual {
		t.Fatalf("cancelled visual r should stay in visual, got %d", m.mode)
	}
}

func TestVisualWordEndExtends(t *testing.T) {
	m := keys(results(t), "v", "e", "d") // select to end of "go"
	wantBuf(t, m, " build ./...")
}

func TestVisualFindExtends(t *testing.T) {
	m := keys(results(t), "v", "f", "b", "d") // select through 'b'
	wantBuf(t, m, "uild ./...")
}

func TestVisualToggleCase(t *testing.T) {
	m := keys(results(t), "v", "l", "~")
	wantBuf(t, m, "GO build ./...")
	if m.mode != modeNormal {
		t.Fatalf("visual ~ should return to normal, got %d", m.mode)
	}
}

// ---------------------------------------------------------------------------
// Mode interrupts: escape out of each mode
// ---------------------------------------------------------------------------

func TestEscFromInsertReturnsToNormal(t *testing.T) {
	m := keys(results(t), "i", "esc")
	if m.mode != modeNormal || m.focus != focusResults {
		t.Fatalf("esc from insert: mode=%d focus=%d, want normal/results", m.mode, m.focus)
	}
}

func TestEscFromInsertKeepsTypedText(t *testing.T) {
	m := keys(results(t), "I", "X", "esc")
	wantBuf(t, m, "Xgo build ./...")
	if m.mode != modeNormal {
		t.Fatalf("want normal mode after esc, got %d", m.mode)
	}
}

func TestEscFromVisualReturnsToNormal(t *testing.T) {
	m := keys(results(t), "v", "l", "l", "esc")
	if m.mode != modeNormal || m.focus != focusResults {
		t.Fatalf("esc from visual: mode=%d focus=%d, want normal/results", m.mode, m.focus)
	}
}

func TestDoubleEscFromInsertReachesSearch(t *testing.T) {
	// First esc: insert -> normal. Second esc: normal -> search.
	m := keys(results(t), "i", "esc", "esc")
	if m.focus != focusSearch {
		t.Fatalf("double esc should land in search focus, got %d", m.focus)
	}
}

func TestDoubleEscFromVisualReachesSearch(t *testing.T) {
	m := keys(results(t), "v", "esc", "esc")
	if m.focus != focusSearch {
		t.Fatalf("double esc from visual should reach search, got %d", m.focus)
	}
}

func TestEscFromNormalReturnsToSearchMode(t *testing.T) {
	m := keys(results(t), "esc")
	if m.focus != focusSearch {
		t.Fatalf("esc from normal should focus search, got %d", m.focus)
	}
}

func TestEscFromCommandCancelsLine(t *testing.T) {
	m := keys(results(t), ":", "w", "esc")
	if m.commandMode {
		t.Fatalf("esc should leave command mode")
	}
	if m.commandLine != "" {
		t.Fatalf("esc should clear command line, got %q", m.commandLine)
	}
	if m.focus != focusResults {
		t.Fatalf("esc from command should stay in results, got %d", m.focus)
	}
	if m.executed != "" {
		t.Fatalf("cancelled command must not run, executed=%q", m.executed)
	}
}

func TestEscFromAliasClosesBox(t *testing.T) {
	m := keys(results(t), "m", "esc")
	if m.aliasing {
		t.Fatalf("esc should close the alias box")
	}
	if m.focus != focusResults {
		t.Fatalf("esc from alias should stay in results, got %d", m.focus)
	}
}

// ---------------------------------------------------------------------------
// Pending-state interrupts (count / g / operator cancelled by another key)
// ---------------------------------------------------------------------------

func TestEscCancelsPendingG(t *testing.T) {
	// `g` arms a pending gg; any non-g key (here esc) cancels it WITHOUT
	// leaving results — selection and focus stay put.
	m := keys(results(t), "j", "g", "esc")
	if m.focus != focusResults {
		t.Fatalf("esc after g should stay in results, got %d", m.focus)
	}
	if m.gPending {
		t.Fatalf("pending g should be cleared")
	}
	if m.selectedIndex != 1 {
		t.Fatalf("selection should be unchanged at 1, got %d", m.selectedIndex)
	}
}

func TestEscCancelsPendingOperator(t *testing.T) {
	// `d` arms an operator; esc is not a motion, so it cancels with no edit.
	m := keys(results(t), "d", "esc")
	wantBuf(t, m, cmd0)
	if m.pending != "" {
		t.Fatalf("pending operator should clear, got %q", m.pending)
	}
	if m.focus != focusResults {
		t.Fatalf("operator-cancel should stay in results, got %d", m.focus)
	}
}

func TestEscDiscardsPendingCountAndLeaves(t *testing.T) {
	// A half-typed count is discarded by esc, which then leaves to search.
	m := keys(results(t), "2", "esc")
	if m.focus != focusSearch {
		t.Fatalf("esc with pending count should reach search, got %d", m.focus)
	}
	if m.count != "" {
		t.Fatalf("pending count should be cleared, got %q", m.count)
	}
}

// ---------------------------------------------------------------------------
// Ex command line (:w :wq :x :q :q!) — typing, dispatch, errors
// ---------------------------------------------------------------------------

func TestCommandLineBuildsAndBackspaces(t *testing.T) {
	m := keys(results(t), ":", "w", "q")
	if m.commandLine != "wq" {
		t.Fatalf("command line = %q, want %q", m.commandLine, "wq")
	}
	m = keys(m, "backspace")
	if m.commandLine != "w" {
		t.Fatalf("after backspace = %q, want %q", m.commandLine, "w")
	}
}

func TestBackspacePastStartLeavesCommandMode(t *testing.T) {
	m := keys(results(t), ":", "backspace")
	if m.commandMode {
		t.Fatalf("backspace on empty command line should exit command mode")
	}
}

func TestWriteStaysOpenAndClearsDirty(t *testing.T) {
	m := keys(results(t), "d", "d", ":", "w", "enter")
	if m.dirty {
		t.Fatalf(":w should clear dirty")
	}
	if m.statusMsg != "written" {
		t.Fatalf(":w status = %q, want 'written'", m.statusMsg)
	}
	if m.executed != "" {
		t.Fatalf(":w must not quit/run, executed=%q", m.executed)
	}
}

func TestWriteQuitSavesAndClearsDirty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	m := keys(*New([]string{cmd1, cmd0}, core.AliasIndex{}), "ctrl+j", "d", "d", ":", "w", "q", "enter")
	if m.dirty {
		t.Fatalf(":wq should clear dirty")
	}
	reloaded := *New([]string{cmd1, cmd0}, core.AliasIndex{})
	if len(reloaded.commands) != 1 {
		t.Fatalf(":wq should persist deletion, got %v", reloaded.commands)
	}
}

func TestXIsWriteQuitAlias(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	keys(*New([]string{cmd1, cmd0}, core.AliasIndex{}), "ctrl+j", "d", "d", ":", "x", "enter")
	reloaded := *New([]string{cmd1, cmd0}, core.AliasIndex{})
	if len(reloaded.commands) != 1 {
		t.Fatalf(":x should persist like :wq, got %v", reloaded.commands)
	}
}

func TestCleanQuitDoesNotWarn(t *testing.T) {
	m := keys(results(t), ":", "q", "enter")
	if m.statusMsg != "" {
		t.Fatalf("clean :q should not warn, got %q", m.statusMsg)
	}
}

func TestUnknownExCommandReportsError(t *testing.T) {
	m := keys(results(t), ":", "z", "z", "z", "enter")
	if m.statusMsg == "" {
		t.Fatalf("unknown :command should set an error status")
	}
	if m.commandMode {
		t.Fatalf("command mode should close after dispatch")
	}
	if m.executed != "" {
		t.Fatalf("unknown command must not run, executed=%q", m.executed)
	}
}

// ---------------------------------------------------------------------------
// End-to-end mode round-trips
// ---------------------------------------------------------------------------

func TestRoundTripSearchResultsInsertSearch(t *testing.T) {
	m := newModel(t)
	if m.focus != focusSearch {
		t.Fatalf("start in search, got %d", m.focus)
	}
	m = keys(m, "ctrl+j") // -> results/normal
	m = keys(m, "i", "X") // -> insert, type
	m = keys(m, "esc")    // -> normal
	if m.mode != modeNormal {
		t.Fatalf("expected normal after esc, got %d", m.mode)
	}
	m = keys(m, "esc") // -> search
	if m.focus != focusSearch {
		t.Fatalf("expected search after second esc, got %d", m.focus)
	}
}

func TestEnterFromInsertRunsEditedCommand(t *testing.T) {
	m := keys(results(t), "A", " ", "-", "v", "enter")
	if m.executed != "go build ./... -v" {
		t.Fatalf("enter from insert should run edited buffer, got %q", m.executed)
	}
}

func TestEnterFromVisualRunsBuffer(t *testing.T) {
	m := keys(results(t), "v", "l", "enter")
	if m.executed != cmd0 {
		t.Fatalf("enter from visual should run buffer, got %q", m.executed)
	}
}

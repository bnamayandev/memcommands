package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetFuzzyScoreListMatchesAliasForExpandedCommand(t *testing.T) {
	history := []string{"neofetch --stdout"}
	aliases := AliasIndex{
		ByAlias: map[string]string{
			"nf": "neofetch",
		},
		ByCommand: map[string][]string{
			"neofetch": {"nf"},
		},
	}

	scored := GetFuzzyScoreList(history, "nf", aliases)
	if len(scored) != 1 {
		t.Fatalf("expected one match, got %d", len(scored))
	}

	if scored[0].Command != "neofetch --stdout" {
		t.Fatalf("expected original command to be preserved, got %q", scored[0].Command)
	}
}

func TestGetFuzzyScoreListMatchesExpandedCommandForAliasHistory(t *testing.T) {
	history := []string{"nf --stdout"}
	aliases := AliasIndex{
		ByAlias: map[string]string{
			"nf": "neofetch",
		},
		ByCommand: map[string][]string{
			"neofetch": {"nf"},
		},
	}

	scored := GetFuzzyScoreList(history, "neofetch", aliases)
	if len(scored) != 1 {
		t.Fatalf("expected one match, got %d", len(scored))
	}

	if scored[0].Command != "nf --stdout" {
		t.Fatalf("expected original command to be preserved, got %q", scored[0].Command)
	}
}

func TestExpandAliasCommandExpandsLeadingAlias(t *testing.T) {
	aliases := AliasIndex{
		ByAlias: map[string]string{
			"nf": "neofetch",
		},
	}

	got := ExpandAliasCommand("nf --stdout", aliases)
	if got != "neofetch --stdout" {
		t.Fatalf("expected alias expansion, got %q", got)
	}
}

func TestExpandAliasCommandLeavesPlainCommandUntouched(t *testing.T) {
	aliases := AliasIndex{
		ByAlias: map[string]string{
			"nf": "neofetch",
		},
	}

	got := ExpandAliasCommand("ls -la", aliases)
	if got != "ls -la" {
		t.Fatalf("expected command to remain unchanged, got %q", got)
	}
}

func TestGetFuzzyScoreListEmptyQueryPrefersMostRecentCommands(t *testing.T) {
	history := []string{
		"git status",
		"ls",
		"git status",
		"pwd",
	}

	scored := GetFuzzyScoreList(history, "", AliasIndex{})
	if len(scored) != 3 {
		t.Fatalf("expected three unique commands, got %d", len(scored))
	}

	want := []string{"pwd", "git status", "ls"}
	for i, command := range want {
		if scored[i].Command != command {
			t.Fatalf("expected command %d to be %q, got %q", i, command, scored[i].Command)
		}
	}
}

func TestNormalizeHistoryLineStripsBashHistoryNumbers(t *testing.T) {
	got := normalizeHistoryLine("  123  git status", "bash")
	if got != "git status" {
		t.Fatalf("expected bash history numbering to be removed, got %q", got)
	}
}

func TestReadFirstAvailableHistoryUsesSingleChronologicalSource(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "primary_history")
	fallback := filepath.Join(dir, "fallback_history")

	if err := os.WriteFile(primary, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatalf("failed to write primary history: %v", err)
	}
	if err := os.WriteFile(fallback, []byte("older\ncommands\n"), 0o644); err != nil {
		t.Fatalf("failed to write fallback history: %v", err)
	}

	lines, err := readFirstAvailableHistory([]historyFile{
		{path: primary, format: "bash"},
		{path: fallback, format: "bash"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected primary history only, got %d lines", len(lines))
	}
	if lines[0] != "first" || lines[1] != "second" {
		t.Fatalf("expected primary history ordering to be preserved, got %#v", lines)
	}
}

func TestGetFuzzyScoreListMatchesUserAlias(t *testing.T) {
	history := []string{`git commit -m "wip"`, "ls -la"}
	aliases := AliasIndex{
		ByFullCommand: BuildUserAliasIndex(map[string]string{
			"gcm": `git commit -m "wip"`,
		}),
	}

	scored := GetFuzzyScoreList(history, "gcm", aliases)
	if len(scored) == 0 {
		t.Fatalf("expected the aliased command to match, got none")
	}
	if scored[0].Command != `git commit -m "wip"` {
		t.Fatalf("expected the original command to surface, got %q", scored[0].Command)
	}
}

func TestUserAliasRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	in := map[string]string{"gcm": `git commit -m "wip"`}
	if err := SaveUserAliases(in); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	out := LoadUserAliases()
	if out["gcm"] != in["gcm"] {
		t.Fatalf("round trip mismatch: got %q", out["gcm"])
	}

	if _, err := os.Stat(filepath.Join(dir, "memcommands", "aliases.json")); err != nil {
		t.Fatalf("expected aliases file to exist: %v", err)
	}
}

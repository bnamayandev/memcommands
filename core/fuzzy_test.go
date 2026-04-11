package core

import "testing"

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

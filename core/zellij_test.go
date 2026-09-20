package core

import "testing"

func TestParseSessionList(t *testing.T) {
	text := "nautilus_trader [Created 3months 6days 4h 27m 55s ago] (EXITED - attach to resurrect)\n" +
		"charming-weasel [Created 15m 9s ago] (current)\n" +
		"ztest [Created 0s ago] \n"

	got := parseSessionList(text)
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions, got %d: %#v", len(got), got)
	}

	want := []Session{
		{Name: "nautilus_trader", Created: "3months 6days 4h 27m 55s", Exited: true, Current: false},
		{Name: "charming-weasel", Created: "15m 9s", Exited: false, Current: true},
		{Name: "ztest", Created: "0s", Exited: false, Current: false},
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("session %d: got %#v, want %#v", i, got[i], w)
		}
	}
}

func TestParseSessionListEmpty(t *testing.T) {
	if got := parseSessionList("No active zellij sessions found.\n"); got != nil {
		t.Fatalf("expected no sessions, got %#v", got)
	}
	if got := parseSessionList(""); got != nil {
		t.Fatalf("expected no sessions, got %#v", got)
	}
}

func TestParseSessionListIgnoresMalformedLines(t *testing.T) {
	got := parseSessionList("garbage line with no marker\ngood [Created 1s ago] (current)\n")
	if len(got) != 1 || got[0].Name != "good" {
		t.Fatalf("expected only the well-formed line, got %#v", got)
	}
}

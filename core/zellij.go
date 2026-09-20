// Package core's zellij helpers shell out to the zellij CLI to list, attach,
// rename, kill, and delete sessions. There's no Go client library for zellij —
// the CLI is the only interface.
package core

import (
	"fmt"
	"os/exec"
	"strings"
)

// ZellijPath resolves the zellij binary on PATH, with a clear error if it's
// not installed.
func ZellijPath() (string, error) {
	path, err := exec.LookPath("zellij")
	if err != nil {
		return "", fmt.Errorf("zellij not found on PATH — install zellij first")
	}
	return path, nil
}

// Session is one row of `zellij list-sessions`.
type Session struct {
	Name    string
	Created string
	// Exited sessions have no live process; attaching to one resurrects it,
	// but renaming one isn't possible (no live IPC socket to target).
	Exited bool
	// Current is the session this terminal is presently attached to, if any.
	Current bool
}

// ListSessions runs `zellij list-sessions -n` and parses its stable,
// unformatted output. An empty result (no active sessions) is not an error.
func ListSessions() ([]Session, error) {
	zellijPath, err := ZellijPath()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(zellijPath, "list-sessions", "-n").CombinedOutput()
	text := string(out)
	if err != nil {
		if strings.Contains(text, "No active zellij sessions") {
			return nil, nil
		}
		return nil, fmt.Errorf("zellij list-sessions: %w: %s", err, strings.TrimSpace(text))
	}

	return parseSessionList(text), nil
}

// parseSessionList parses lines shaped like:
//
//	nautilus_trader [Created 3months 6days 4h 27m 55s ago] (EXITED - attach to resurrect)
//	charming-weasel [Created 15m 9s ago] (current)
//	ztest [Created 0s ago]
//
// Matching is done with substring checks rather than a strict format, so
// minor wording changes across zellij versions don't break parsing.
func parseSessionList(text string) []Session {
	const marker = " [Created "

	var sessions []Session
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}

		name := line[:idx]
		if name == "" {
			continue
		}

		rest := line[idx+len(marker):]
		created := rest
		status := ""
		if end := strings.Index(rest, " ago]"); end >= 0 {
			created = rest[:end]
			status = rest[end+len(" ago]"):]
		}

		sessions = append(sessions, Session{
			Name:    name,
			Created: strings.TrimSpace(created),
			Exited:  strings.Contains(status, "EXITED"),
			Current: strings.Contains(status, "current"),
		})
	}
	return sessions
}

// AttachSession replaces the current process with `zellij attach <name>`,
// creating the session first when create is set. Attaching to an EXITED
// session resurrects it — there's no separate resurrect command.
func AttachSession(name string, create bool) error {
	zellijPath, err := ZellijPath()
	if err != nil {
		return err
	}
	args := []string{"zellij", "attach", name}
	if create {
		args = append(args, "--create")
	}
	return ExecReplace(zellijPath, args)
}

// KillSession ends a running session's processes. Whether it then sticks
// around as a resurrectable EXITED entry (until deleted) or disappears
// outright depends on zellij's own session_serialization setting — this
// call doesn't control that, it just asks zellij to kill it.
func KillSession(name string) error {
	return runZellij("kill-session", name)
}

// DeleteSession permanently removes a session — killing it first if it's
// still running (--force), then purging its resurrect record. This is the
// full, unconditional removal behind zelcommands' `dd`.
func DeleteSession(name string) error {
	return runZellij("delete-session", name, "--force")
}

// RenameSession renames a running session. It targets `current` directly via
// zellij's top-level --session flag, so it works without being attached to
// it — but only for a running session; an EXITED one has no live IPC socket
// to send the action to.
func RenameSession(current, newName string) error {
	zellijPath, err := ZellijPath()
	if err != nil {
		return err
	}
	out, err := exec.Command(zellijPath, "--session", current, "action", "rename-session", newName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", firstLine(out, err))
	}
	return nil
}

func runZellij(args ...string) error {
	zellijPath, err := ZellijPath()
	if err != nil {
		return err
	}
	out, err := exec.Command(zellijPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", firstLine(out, err))
	}
	return nil
}

// firstLine extracts a one-line error message from a CLI failure, preferring
// the command's own output over the generic exec error.
func firstLine(out []byte, fallback error) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return fallback.Error()
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return text
}

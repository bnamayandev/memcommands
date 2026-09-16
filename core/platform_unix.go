//go:build unix

package core

import (
	"os/exec"
	"syscall"
)

// ShellExecutable resolves a $SHELL value to a command exec.Command can run.
// On POSIX systems $SHELL is already a real, absolute path.
func ShellExecutable(shell string) string {
	return shell
}

// DetachFromControllingTTY starts cmd in a new session, keeping its job
// control off the TTY the caller owns — needed when a shell is spawned
// asynchronously alongside an interactive program like Bubble Tea, else the
// spawned shell's job control can corrupt the caller's input.
func DetachFromControllingTTY(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

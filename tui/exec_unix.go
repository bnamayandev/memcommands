//go:build unix

package main

import (
	"os"
	"syscall"
)

const defaultShell = "/bin/sh"

// execShellCommand replaces the current process image with resolvedShell
// running args, so the command inherits sole ownership of stdin/stdout/stderr.
func execShellCommand(resolvedShell string, args []string) error {
	// Bubble Tea can leave the tty non-blocking; clear it so the command's reads block.
	for _, fd := range []int{0, 1, 2} {
		_ = syscall.SetNonblock(fd, false)
	}
	return syscall.Exec(resolvedShell, args, os.Environ())
}

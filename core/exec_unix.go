//go:build unix

package core

import (
	"os"
	"syscall"
)

// ExecReplace replaces the current process image with resolvedPath running
// args, so the target inherits sole ownership of stdin/stdout/stderr.
func ExecReplace(resolvedPath string, args []string) error {
	// Bubble Tea can leave the tty non-blocking; clear it so the target's reads block.
	for _, fd := range []int{0, 1, 2} {
		_ = syscall.SetNonblock(fd, false)
	}
	return syscall.Exec(resolvedPath, args, os.Environ())
}

//go:build windows

package core

import (
	"os/exec"
	"path/filepath"
)

// ShellExecutable resolves a $SHELL value to a command exec.Command can run.
// Under Git Bash/MSYS2, $SHELL is an MSYS-style path like "/usr/bin/bash"
// that Windows' CreateProcess can't resolve directly, so fall back to the
// bare executable name and let PATH lookup — which MSYS already populates
// with its own bin directories — find it.
func ShellExecutable(shell string) string {
	return filepath.Base(shell)
}

// DetachFromControllingTTY is a no-op on Windows: there's no ctty/job-control
// concept to detach a spawned shell from here.
func DetachFromControllingTTY(cmd *exec.Cmd) {}

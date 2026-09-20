//go:build windows

package core

import (
	"errors"
	"os"
	"os/exec"
)

// ExecReplace runs resolvedPath as a child process and exits with its status.
// Windows has no process-image-replacement syscall, so unlike unix this can't
// hand the terminal off directly — it spawns and waits instead.
func ExecReplace(resolvedPath string, args []string) error {
	cmd := exec.Command(resolvedPath, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

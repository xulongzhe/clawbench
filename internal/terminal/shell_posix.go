//go:build !windows

package terminal

import (
	"os/exec"
	"syscall"
)

// killProcessGroupSig sends a signal to the process group of the given command.
// pty.Start creates the shell with Setsid=true, which starts a new session
// and process group — so Getpgid works to find and kill the whole group.
func killProcessGroupSig(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Signal(sig)
		return
	}

	if err := syscall.Kill(-pgid, sig); err != nil {
		_ = cmd.Process.Signal(sig)
	}
}

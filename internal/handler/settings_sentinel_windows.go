//go:build windows

package handler

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

func launchSentinel() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	args := os.Args[1:]

	sentinelScript := fmt.Sprintf(
		"timeout /t 2 /nobreak >nul & %s %s",
		shellQuote(exe), joinArgs(args),
	)
	cmd := exec.Command("cmd", "/c", sentinelScript)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sentinel process: %w", err)
	}

	slog.Info("sentinel process started", "pid", cmd.Process.Pid, "parent_pid", os.Getpid())
	return cmd, nil
}

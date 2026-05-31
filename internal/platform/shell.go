package platform

import (
	"bufio"
	"log/slog"
	"os"
	"os/user"
	"strings"
)

// ResolveLoginShell returns the user's login shell, respecting their system
// configuration. The resolution order is:
//
//  1. $SHELL — if set and not /bin/sh (which is often dash on Debian/Ubuntu;
//     the user's actual login shell is recorded in /etc/passwd).
//  2. /etc/passwd — look up the current user's login shell.
//  3. Fallback to /bin/sh.
//
// On non-POSIX systems (Windows), $SHELL is returned as-is if set, otherwise
// an empty string (callers should fall back to their own Windows logic).
func ResolveLoginShell() string {
	shell := os.Getenv("SHELL")

	// On Windows, $SHELL is usually unset; return whatever we have.
	if IsWindows() {
		return shell
	}

	// If $SHELL is set to something other than /bin/sh, trust it.
	if shell != "" && shell != "/bin/sh" {
		return shell
	}

	// $SHELL is empty or /bin/sh — try /etc/passwd for the real login shell.
	if loginShell := lookupPasswdShell(); loginShell != "" {
		return loginShell
	}

	// Last resort: keep whatever $SHELL was (may be "/bin/sh" or empty).
	if shell != "" {
		return shell
	}
	return "/bin/sh"
}

// lookupPasswdShell reads the current user's login shell from /etc/passwd.
// Returns empty string on any error.
func lookupPasswdShell() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}

	f, err := os.Open("/etc/passwd")
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	prefix := u.Username + ":"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		// Format: username:x:uid:gid:gecos:home:shell
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		passwdUser := parts[0]
		passwdUID := parts[2]
		passwdShell := parts[6]

		// Match by username AND uid to avoid collisions
		if passwdUser == u.Username && passwdUID == u.Uid && passwdShell != "" {
			// Sanity: skip nologin/false shells
			base := passwdShell
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if base == "nologin" || base == "false" || base == "sync" {
				continue
			}
			return passwdShell
		}
	}
	return ""
}

// SetLoginShell ensures the SHELL environment variable reflects the user's
// actual login shell. This is needed because on Debian/Ubuntu, $SHELL may
// be /bin/sh (dash) when the process was started from a non-login context
// (e.g., systemd, cron, or nohup), even though the user's login shell in
// /etc/passwd is /bin/bash or zsh.
//
// AI CLI tools (Claude, CodeBuddy, Codex, etc.) read $SHELL to determine
// which shell to use for their "Bash tool", so an incorrect $SHELL causes
// them to run commands in dash instead of the user's configured shell.
func SetLoginShell() {
	current := os.Getenv("SHELL")
	resolved := ResolveLoginShell()
	if resolved != current {
		slog.Info(
			"correcting SHELL to user's login shell",
			slog.String("old", current),
			slog.String("new", resolved),
		)
		_ = os.Setenv("SHELL", resolved)
	}
}

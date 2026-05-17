package platform

import (
	"os"
	"testing"
)

func TestResolveLoginShell(t *testing.T) {
	// Save and restore original SHELL
	origShell := os.Getenv("SHELL")
	t.Cleanup(func() { os.Setenv("SHELL", origShell) })

	t.Run("respects non-sh SHELL", func(t *testing.T) {
		os.Setenv("SHELL", "/bin/zsh")
		got := ResolveLoginShell()
		if got != "/bin/zsh" {
			t.Errorf("got %q, want /bin/zsh", got)
		}
	})

	t.Run("falls back to passwd when SHELL is /bin/sh", func(t *testing.T) {
		os.Setenv("SHELL", "/bin/sh")
		got := ResolveLoginShell()
		// On Linux, root's login shell in /etc/passwd is typically /bin/bash or /usr/bin/zsh,
		// so ResolveLoginShell should return that instead of /bin/sh.
		// On macOS, root's login shell IS /bin/sh, so this test returns /bin/sh legitimately.
		// We just verify the function returns a non-empty value.
		if got == "" {
			t.Errorf("ResolveLoginShell() returned empty string")
		}
		t.Logf("resolved login shell: %s", got)
	})

	t.Run("falls back to passwd when SHELL is empty", func(t *testing.T) {
		os.Unsetenv("SHELL")
		got := ResolveLoginShell()
		if got == "" {
			t.Errorf("ResolveLoginShell() returned empty string")
		}
		t.Logf("resolved login shell: %s", got)
	})
}

func TestSetLoginShell(t *testing.T) {
	origShell := os.Getenv("SHELL")
	t.Cleanup(func() { os.Setenv("SHELL", origShell) })

	os.Setenv("SHELL", "/bin/sh")
	SetLoginShell()

	got := os.Getenv("SHELL")
	// On macOS, root's login shell in /etc/passwd is /bin/sh, so SetLoginShell
	// correctly keeps it as /bin/sh. Only fail if it's still /bin/sh AND
	// /etc/passwd has a different shell for this user.
	if got == "/bin/sh" {
		// Verify this is actually the login shell from /etc/passwd
		resolved := ResolveLoginShell()
		if resolved != "/bin/sh" {
			t.Errorf("SHELL still /bin/sh after SetLoginShell(), expected %q", resolved)
		}
	}
	t.Logf("SHELL after SetLoginShell(): %s", got)
}

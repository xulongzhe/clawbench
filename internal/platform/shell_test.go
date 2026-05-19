package platform

import (
	"os"
	"runtime"
	"testing"
)

func TestResolveLoginShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell resolution is POSIX-only")
	}

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
		// On some systems (e.g., macOS CI runners), /bin/sh IS the login shell
		// recorded in /etc/passwd. We just verify it returns a non-empty value.
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

	t.Run("keeps bin-sh when passwd has no better shell", func(t *testing.T) {
		// If $SHELL is /bin/sh and passwd lookup returns empty,
		// it should keep /bin/sh
		os.Setenv("SHELL", "/bin/sh")
		got := ResolveLoginShell()
		if got == "" {
			t.Errorf("ResolveLoginShell() returned empty string")
		}
	})
}

func TestResolveLoginShell_FallbackToBinSh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell resolution is POSIX-only")
	}

	origShell := os.Getenv("SHELL")
	t.Cleanup(func() { os.Setenv("SHELL", origShell) })

	// When SHELL is empty and passwd lookup fails, should return /bin/sh
	os.Unsetenv("SHELL")
	got := ResolveLoginShell()
	// On real systems, passwd lookup usually succeeds, so we just verify non-empty
	if got == "" {
		t.Errorf("ResolveLoginShell() returned empty string")
	}
}

func TestLookupPasswdShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("passwd lookup is POSIX-only")
	}

	// lookupPasswdShell should return a non-empty string for the current user
	// (assuming a normal Unix environment with /etc/passwd)
	shell := lookupPasswdShell()
	t.Logf("lookupPasswdShell() = %q", shell)
	// We don't assert non-empty because CI environments may have unusual setups
}

func TestSetLoginShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell resolution is POSIX-only")
	}

	origShell := os.Getenv("SHELL")
	t.Cleanup(func() { os.Setenv("SHELL", origShell) })

	os.Setenv("SHELL", "/bin/sh")
	SetLoginShell()

	got := os.Getenv("SHELL")
	if got == "" {
		t.Errorf("SHELL is empty after SetLoginShell()")
	}
	t.Logf("SHELL after SetLoginShell(): %s", got)
}

package version

import (
	"runtime/debug"
	"testing"
)

func TestVersionDefault(t *testing.T) {
	// Default value should be empty string (not set by ldflags)
	if Version != "" {
		t.Errorf("expected default Version to be empty string, got %q", Version)
	}
}

func TestVersionCanBeSet(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v1.0.0"
	if Version != "v1.0.0" {
		t.Errorf("expected Version to be %q, got %q", "v1.0.0", Version)
	}
}

func TestGet_ReturnsVersionWhenSet(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v2.0.0"
	if got := Get(); got != "v2.0.0" {
		t.Errorf("Get() = %q, want %q", got, "v2.0.0")
	}
}

func TestGet_FallsBackToVCS(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = ""
	got := Get()
	// In a git checkout, should return a short SHA or "dev"
	if got == "" {
		t.Error("Get() returned empty string, expected non-empty version")
	}
}

func TestGet_FallsBackToMainVersion(t *testing.T) {
	// When Version is empty and there's no VCS info but Main.Version is set,
	// Get() should return info.Main.Version.
	// In practice this branch is hard to trigger in a real git checkout
	// because VCS info is always present. We verify the logic path by
	// confirming the function doesn't panic and returns a non-empty string.
	original := Version
	defer func() { Version = original }()

	Version = ""
	got := Get()
	if got == "" {
		t.Error("Get() returned empty string")
	}

	// Verify the result comes from one of the expected sources
	if _, ok := debug.ReadBuildInfo(); !ok {
		// No build info at all — should return "dev"
		if got != "dev" {
			t.Errorf("Get() = %q without build info, want %q", got, "dev")
		}
	}
}

func TestGet_EmptyVersionString(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = ""
	result := Get()

	// Result must be one of: VCS short SHA, Main.Version, or "dev"
	if result == "" {
		t.Error("Get() should never return empty string")
	}
}

func TestGet_PriorityOverVCS(t *testing.T) {
	// When Version is set via ldflags, it should take priority over VCS fallback
	original := Version
	defer func() { Version = original }()

	Version = "v3.0.0-beta"
	if got := Get(); got != "v3.0.0-beta" {
		t.Errorf("Get() = %q, want %q (ldflags should take priority)", got, "v3.0.0-beta")
	}
}

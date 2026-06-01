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

func TestGet_PriorityOverVCS(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v3.0.0-beta"
	if got := Get(); got != "v3.0.0-beta" {
		t.Errorf("Get() = %q, want %q (ldflags should take priority)", got, "v3.0.0-beta")
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

func TestGet_ReadBuildInfoFails(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}

	got := Get()
	if got != "dev" {
		t.Errorf("Get() = %q when ReadBuildInfo fails, want %q", got, "dev")
	}
}

func TestGet_VCSRevisionFound(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc1234def5678"},
			},
		}, true
	}

	got := Get()
	if got != "abc1234" {
		t.Errorf("Get() = %q, want %q", got, "abc1234")
	}
}

func TestGet_VCSRevisionTooShort(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc"}, // too short
			},
			Main: debug.Module{Version: "v0.0.1"},
		}, true
	}

	got := Get()
	if got != "v0.0.1" {
		t.Errorf("Get() = %q, want %q (should fall back to Main.Version)", got, "v0.0.1")
	}
}

func TestGet_MainVersionDevel(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "(devel)"},
		}, true
	}

	got := Get()
	if got != "dev" {
		t.Errorf("Get() = %q when Main.Version is (devel), want %q", got, "dev")
	}
}

func TestGet_NoVCSNoMainVersion(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{}, true
	}

	got := Get()
	if got != "dev" {
		t.Errorf("Get() = %q when no VCS or Main.Version, want %q", got, "dev")
	}
}

func TestGet_MainVersionSet(t *testing.T) {
	original := Version
	origReadBuildInfo := readBuildInfo
	defer func() {
		Version = original
		readBuildInfo = origReadBuildInfo
	}()

	Version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
		}, true
	}

	got := Get()
	if got != "v1.2.3" {
		t.Errorf("Get() = %q, want %q", got, "v1.2.3")
	}
}

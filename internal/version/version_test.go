package version

import "testing"

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

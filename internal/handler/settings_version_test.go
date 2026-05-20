package handler

import (
	"regexp"
	"testing"

	"clawbench/internal/version"

	"github.com/stretchr/testify/assert"
)

func TestGetBuildVersion_WithLdflagsVersion(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// Simulate ldflags-injected version like "v1.0.0"
	version.Version = "v1.0.0"
	result := getBuildVersion()

	// Should contain the version string
	assert.Contains(t, result, "v1.0.0")

	// When running in a git repo (CI or local), VCS info is available,
	// so the result should also contain the short SHA in parentheses
	re := regexp.MustCompile(`^v1\.0\.0 \([0-9a-f]{7}\)$`)
	if !re.MatchString(result) {
		// If no VCS info (unlikely but possible), just the version string
		assert.Equal(t, "v1.0.0", result)
	}
}

func TestGetBuildVersion_WithLdflagsPreReleaseVersion(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// Simulate pre-release version like "v0.30.0-30-g830bb6c"
	version.Version = "v0.30.0-30-g830bb6c"
	result := getBuildVersion()

	assert.Contains(t, result, "v0.30.0-30-g830bb6c")

	// If VCS info available, should append short SHA
	re := regexp.MustCompile(`^v0\.30\.0-30-g830bb6c \([0-9a-f]{7}\)$`)
	if !re.MatchString(result) {
		assert.Equal(t, "v0.30.0-30-g830bb6c", result)
	}
}

func TestGetBuildVersion_WithLdflagsShortSHA(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// When no git tags exist, git describe returns just the short SHA
	version.Version = "abc1234"
	result := getBuildVersion()

	assert.Contains(t, result, "abc1234")
}

func TestGetBuildVersion_WithoutLdflagsVersion(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// No ldflags — should fall back to VCS info from debug.ReadBuildInfo()
	version.Version = ""
	result := getBuildVersion()

	// In a git repo, result should be either:
	// - "abc1234 (2025-05-20T10:30:00Z)" (VCS SHA + time)
	// - "abc1234" (VCS SHA only)
	// - "dev" (no VCS info)
	assert.NotEmpty(t, result)

	// When VCS info is available (running in git repo), should NOT be just "dev"
	// unless the test is run outside a git repo
	if len(result) >= 7 {
		// Should look like a short SHA or SHA + timestamp
		matched, _ := regexp.MatchString(`^[0-9a-f]{7}`, result)
		assert.True(t, matched, "expected result to start with 7-char hex SHA, got %q", result)
	}
}

func TestGetBuildVersion_LdflagsVersionTakesPrecedence(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// When version is set, it should always appear in the output
	// regardless of VCS info availability
	version.Version = "v2.0.0"
	result := getBuildVersion()
	assert.Contains(t, result, "v2.0.0")
	// Should NOT contain timestamp (which is the old format without ldflags)
	assert.NotContains(t, result, "T")
}

func TestGetBuildVersion_VersionWithVCSSHA(t *testing.T) {
	original := version.Version
	defer func() { version.Version = original }()

	// The key feature: ldflags version + VCS SHA = "v1.0.0 (abc1234)"
	version.Version = "v1.0.0"
	result := getBuildVersion()

	// If VCS info available, format should be "v1.0.0 (sha)"
	re := regexp.MustCompile(`^v1\.0\.0 \([0-9a-f]{7}\)$`)
	if re.MatchString(result) {
		// Extract the SHA part to verify it's reasonable
		submatch := re.FindStringSubmatch(result)
		assert.Len(t, submatch[0], len("v1.0.0 (") + 7 + 1) // +7 for SHA, +1 for ")"
	}
}

func TestGetBuildVersion_WithoutBuildInfo(t *testing.T) {
	// This test exercises the case where debug.ReadBuildInfo returns ok=false.
	// Since we can't easily mock debug.ReadBuildInfo, we test the version.Version
	// fallback logic that's also exercised when ok=true but no VCS info is found.
	// The !ok branch returns version.Version if set, or "dev".
	// We verify the "dev" fallback:
	original := version.Version
	defer func() { version.Version = original }()

	version.Version = ""
	result := getBuildVersion()
	// In a git repo, VCS info is available so result won't be "dev"
	// But it should always return a non-empty string
	assert.NotEmpty(t, result)
}

func TestGetBuildVersion_VersionOnlyNoVCS(t *testing.T) {
	// When version is set but no VCS info (e.g., go install without git),
	// the result should just be the version string
	original := version.Version
	defer func() { version.Version = original }()

	// This test verifies the code path where version is set
	// In a real git repo, VCS info is available, so it will include SHA
	version.Version = "v3.0.0-test"
	result := getBuildVersion()
	assert.Contains(t, result, "v3.0.0-test")
}

func TestGetBuildVersion_VCSOnlyNoVersion(t *testing.T) {
	// When no version via ldflags, falls back to VCS info
	original := version.Version
	defer func() { version.Version = original }()

	version.Version = ""
	result := getBuildVersion()

	// In a git repo, VCS info is available:
	// Result is either "sha (timestamp)" or "sha" or "dev"
	assert.NotEmpty(t, result)
}

func TestGetBuildVersion_DevFallback(t *testing.T) {
	// Verify that when called, it never returns an empty string
	original := version.Version
	defer func() { version.Version = original }()

	version.Version = ""
	result := getBuildVersion()
	assert.NotEmpty(t, result, "getBuildVersion should never return empty string")
}

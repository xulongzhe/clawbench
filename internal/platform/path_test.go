package platform

import (
	"runtime"
	"testing"
)

func TestIsWindows(t *testing.T) {
	result := IsWindows()
	expected := runtime.GOOS == "windows"
	if result != expected {
		t.Errorf("IsWindows() = %v, want %v", result, expected)
	}
}

func TestTempDir(t *testing.T) {
	dir := TempDir()
	if dir == "" {
		t.Error("TempDir() returned empty string")
	}
}

func TestUserHomeDir(t *testing.T) {
	home := UserHomeDir()
	if home == "" {
		t.Error("UserHomeDir() returned empty string")
	}
}

func TestClaudeConfigDir(t *testing.T) {
	dir := ClaudeConfigDir()
	if dir == "" {
		t.Error("ClaudeConfigDir() returned empty string")
	}
	// Should end with .claude
	if len(dir) < 7 || dir[len(dir)-7:] != ".claude" {
		t.Errorf("ClaudeConfigDir() = %q, want path ending with .claude", dir)
	}
}

func TestManglePathForOS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		goos     string
		expected string
	}{
		// Unix paths
		{
			name:     "unix path on linux",
			input:    "/home/user/project",
			goos:     "linux",
			expected: "-home-user-project",
		},
		{
			name:     "root path on linux",
			input:    "/",
			goos:     "linux",
			expected: "-",
		},
		// Windows paths (always testable regardless of runtime OS)
		{
			name:     "windows path on windows",
			input:    `C:\Users\user\project`,
			goos:     "windows",
			expected: "C--Users-user-project",
		},
		{
			name:     "windows drive root",
			input:    `C:\`,
			goos:     "windows",
			expected: "C--",
		},
		{
			name:     "windows drive only",
			input:    `C:`,
			goos:     "windows",
			expected: "C-",
		},
		// Edge cases
		{
			name:     "mixed separators treated as windows",
			input:    `C:\Users/admin\project`,
			goos:     "windows",
			expected: "C--Users-admin-project",
		},
		{
			name:     "unix path on windows no drive",
			input:    "/home/user/project",
			goos:     "windows",
			expected: "-home-user-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ManglePathForOS(tt.input, tt.goos)
			if result != tt.expected {
				t.Errorf("ManglePathForOS(%q, %q) = %q, want %q", tt.input, tt.goos, result, tt.expected)
			}
		})
	}
}

func TestManglePath(t *testing.T) {
	// Test that ManglePath delegates correctly to ManglePathForOS with runtime.GOOS
	result := ManglePath("/home/user/project")
	expected := ManglePathForOS("/home/user/project", runtime.GOOS)
	if result != expected {
		t.Errorf("ManglePath(%q) = %q, want %q (same as ManglePathForOS with runtime.GOOS)", "/home/user/project", result, expected)
	}
}

func TestExpandTilde(t *testing.T) {
	home := UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde with slash",
			input:    "~/Documents",
			expected: home + "/Documents",
		},
		{
			name:     "bare tilde",
			input:    "~",
			expected: home,
		},
		{
			name:     "absolute path unchanged",
			input:    "/home/user/project",
			expected: "/home/user/project",
		},
		{
			name:     "relative path unchanged",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Windows backslash tilde
	if runtime.GOOS == "windows" {
		result := ExpandTilde(`~\Documents`)
		expected := home + `\Documents`
		if result != expected {
			t.Errorf("ExpandTilde(%q) = %q, want %q", `~\Documents`, result, expected)
		}
	}
}

func TestClaudeProjectDir(t *testing.T) {
	dir := ClaudeProjectDir("/home/user/project")
	if dir == "" {
		t.Error("ClaudeProjectDir() returned empty string")
	}
	// Should contain "projects" segment
	if len(dir) < 8 {
		t.Errorf("ClaudeProjectDir() = %q, too short", dir)
	}
}

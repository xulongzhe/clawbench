// Package platform provides cross-platform utility functions for path resolution.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsWindows returns true when running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// TempDir returns the OS-appropriate temporary directory.
// On Unix: typically /tmp
// On Windows: typically %TEMP% or %TMP% (e.g. C:\Users\xxx\AppData\Local\Temp)
func TempDir() string {
	return os.TempDir()
}

// UserHomeDir returns the user's home directory in a cross-platform way.
func UserHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback: try environment variables
		if IsWindows() {
			if home = os.Getenv("USERPROFILE"); home != "" {
				return home
			}
		} else {
			home = os.Getenv("HOME")
		}
	}
	return home
}

// ClaudeConfigDir returns the directory where Claude CLI stores configuration.
// All platforms: ~/.claude/
func ClaudeConfigDir() string {
	return filepath.Join(UserHomeDir(), ".claude")
}

// ClaudeProjectDir returns the session directory for a given project path.
// On Unix:    ~/.claude/projects/-home-user-project/
// On Windows: ~/.claude/projects/-C-Users-user-project/
//
// Claude CLI mangles the absolute path by replacing path separators with "-".
// On Windows both "/" and "\" need to be replaced.
//
// Note: Under WSL, Claude runs as a native Windows binary and mangles paths
// using Windows conventions, but this function uses the current Go binary's
// runtime.GOOS. If running under WSL with a Linux binary while Claude runs
// natively on Windows, the mangling may not match.
func ClaudeProjectDir(projectPath string) string {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		absPath = projectPath
	}
	mangled := ManglePath(absPath)
	return filepath.Join(ClaudeConfigDir(), "projects", mangled)
}

// ExpandTilde expands a leading ~/ in a path to the user's home directory.
// On Windows, ~\ is also handled.
// If the path does not start with ~, it is returned unchanged.
func ExpandTilde(path string) string {
	if path == "" {
		return path
	}
	// Handle ~/ (Unix) and ~\ (Windows)
	if len(path) >= 2 && path[0] == '~' && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(UserHomeDir(), path[2:])
	}
	// Handle bare ~
	if path == "~" {
		return UserHomeDir()
	}
	return path
}

// ManglePath converts an absolute path into Claude's mangled directory name.
// All path separators (/ and \) are replaced with "-".
// Drive letters on Windows (e.g. "C:") become "C-" in the result.
func ManglePath(absPath string) string {
	return ManglePathForOS(absPath, runtime.GOOS)
}

// ManglePathForOS converts an absolute path into Claude's mangled directory name
// for a specific OS. This is useful for testing and for WSL scenarios where
// the binary's runtime.GOOS differs from the Claude CLI's platform.
func ManglePathForOS(absPath string, goos string) string {
	// Replace all backslashes with forward slashes first
	normalized := strings.ReplaceAll(absPath, "\\", "/")
	// Replace all forward slashes with dashes
	mangled := strings.ReplaceAll(normalized, "/", "-")
	// On Windows, replace colon after drive letter
	if goos == "windows" && len(mangled) > 1 && mangled[1] == ':' {
		mangled = mangled[:1] + "-" + mangled[2:]
	}
	return mangled
}
